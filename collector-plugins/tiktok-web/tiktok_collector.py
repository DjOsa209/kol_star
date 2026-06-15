#!/usr/bin/env python3
import argparse
import json
import sys
from datetime import datetime, timezone
from urllib import error, parse, request


def main():
    args = parse_args()
    config = load_config(args.config)
    collector_base_url = option(args.collector_base_url, config, "collector_base_url").rstrip("/")
    server_url = option(args.server_url, config, "server_url").rstrip("/")
    token = option(args.token, config, "token")
    post_count = int(option(args.post_count, config, "post_count", 35))

    username = normalize_username(args.username or username_from_url(args.creator_url))
    sec_uid = args.sec_uid or ""
    if not username and not sec_uid and args.creator_url:
        sec_uid = collector_get(collector_base_url, "get_sec_user_id", {"url": args.creator_url})
    if not username and args.creator_url:
        username = collector_get(collector_base_url, "get_unique_id", {"url": args.creator_url})
    if not username and not sec_uid:
        fail("provide --creator-url, --username, or --sec-uid")

    profile_data = unwrap_data(collector_get(
        collector_base_url,
        "fetch_user_profile",
        {"uniqueId": username or "", "secUid": sec_uid or ""},
    ))
    creator = normalize_creator(profile_data, username, sec_uid)
    sec_uid = creator.get("secUid") or sec_uid

    posts = []
    raw_posts = {}
    if sec_uid:
        raw_posts = unwrap_data(collector_get(
            collector_base_url,
            "fetch_user_post",
            {"secUid": sec_uid, "count": post_count, "cursor": 0, "coverFormat": 2},
        ))
        posts = normalize_posts(raw_posts, creator.get("username", username))

    payload = {
        "collector": "tiktok-web",
        "collectedAt": datetime.now(timezone.utc).isoformat(),
        "resourceId": args.resource_id or 0,
        "creator": creator,
        "posts": posts,
    }
    if args.include_raw:
        payload["raw"] = {"profile": profile_data, "posts": raw_posts}

    result = post_json(
        server_url + "/collector/tiktok/callback",
        payload,
        {"X-Collector-Token": token},
    )
    print(json.dumps(result, ensure_ascii=False, indent=2))


def parse_args():
    parser = argparse.ArgumentParser(description="Collect TikTok creator data and callback kol_admin.")
    parser.add_argument("--config", default="", help="Path to config json.")
    parser.add_argument("--creator-url", default="", help="TikTok profile URL.")
    parser.add_argument("--username", default="", help="TikTok uniqueId without @.")
    parser.add_argument("--sec-uid", default="", help="TikTok secUid.")
    parser.add_argument("--resource-id", type=int, default=0, help="Existing kol_admin resource id.")
    parser.add_argument("--collector-base-url", default="", help="Douyin_TikTok_Download_API TikTok web base URL.")
    parser.add_argument("--server-url", default="", help="kol_admin backend URL.")
    parser.add_argument("--token", default="", help="Collector callback token.")
    parser.add_argument("--post-count", default="", help="Number of recent posts to collect.")
    parser.add_argument("--include-raw", action="store_true", help="Include raw collector payload in callback.")
    return parser.parse_args()


def load_config(path):
    if not path:
        return {}
    with open(path, "r", encoding="utf-8") as file:
        return json.load(file)


def option(value, config, key, default=""):
    if value not in ("", None):
        return value
    return config.get(key, default)


def collector_get(base_url, endpoint, params):
    query = parse.urlencode({k: v for k, v in params.items() if v not in ("", None)})
    url = f"{base_url}/{endpoint}"
    if query:
        url += "?" + query
    return get_json(url)


def get_json(url):
    try:
        with request.urlopen(url, timeout=30) as response:
            return json.loads(response.read().decode("utf-8"))
    except error.HTTPError as exc:
        body = exc.read().decode("utf-8", errors="replace")
        fail(f"GET {url} failed: {exc.code} {body[:500]}")
    except Exception as exc:
        fail(f"GET {url} failed: {exc}")


def post_json(url, payload, headers):
    data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
    req = request.Request(
        url,
        data=data,
        headers={"Content-Type": "application/json", **headers},
        method="POST",
    )
    try:
        with request.urlopen(req, timeout=30) as response:
            return json.loads(response.read().decode("utf-8"))
    except error.HTTPError as exc:
        body = exc.read().decode("utf-8", errors="replace")
        fail(f"POST {url} failed: {exc.code} {body[:500]}")
    except Exception as exc:
        fail(f"POST {url} failed: {exc}")


def unwrap_data(payload):
    if isinstance(payload, dict) and "data" in payload:
        return payload["data"]
    return payload


def normalize_creator(data, fallback_username, fallback_sec_uid):
    data = data or {}
    user_info = data.get("userInfo", data)
    user = user_info.get("user", user_info.get("user_info", {}))
    stats = user_info.get("stats", user_info.get("statsV2", {}))
    username = normalize_username(first(user.get("uniqueId"), user.get("unique_id"), fallback_username))
    return {
        "username": username,
        "secUid": first(user.get("secUid"), user.get("sec_uid"), fallback_sec_uid),
        "userId": first(user.get("id"), user.get("uid")),
        "name": first(user.get("nickname"), user.get("displayName"), username),
        "avatarUrl": first(user.get("avatarLarger"), user.get("avatarMedium"), user.get("avatarThumb")),
        "bio": first(user.get("signature"), user.get("bio")),
        "profileUrl": f"https://www.tiktok.com/@{username}" if username else "",
        "followerCount": number(first(stats.get("followerCount"), stats.get("follower_count"))),
        "followingCount": number(first(stats.get("followingCount"), stats.get("following_count"))),
        "likesCount": number(first(stats.get("heartCount"), stats.get("diggCount"), stats.get("likes_count"))),
        "videoCount": number(first(stats.get("videoCount"), stats.get("video_count"))),
    }


def normalize_posts(data, username):
    items = []
    if isinstance(data, dict):
        items = data.get("itemList") or data.get("items") or data.get("aweme_list") or []
    posts = []
    for item in items:
        if not isinstance(item, dict):
            continue
        stats = item.get("stats") or item.get("statsV2") or {}
        video = item.get("video") or {}
        author = item.get("author") or {}
        post_id = str(first(item.get("id"), item.get("aweme_id"), ""))
        post_username = normalize_username(first(author.get("uniqueId"), username))
        if not post_id:
            continue
        posts.append({
            "id": post_id,
            "username": post_username,
            "title": first(item.get("title"), trim(first(item.get("desc"), ""), 120)),
            "description": first(item.get("desc"), item.get("description")),
            "url": first(item.get("shareUrl"), video_url(post_username, post_id)),
            "coverUrl": first(video.get("cover"), video.get("dynamicCover"), video.get("originCover")),
            "mediaType": "VIDEO",
            "publishedAt": number(first(item.get("createTime"), item.get("create_time"))),
            "durationSeconds": number(video.get("duration")) // 1000 if number(video.get("duration")) > 1000 else number(video.get("duration")),
            "viewCount": number(first(stats.get("playCount"), stats.get("viewCount"))),
            "likeCount": number(first(stats.get("diggCount"), stats.get("likeCount"))),
            "commentCount": number(stats.get("commentCount")),
            "shareCount": number(stats.get("shareCount")),
        })
    return posts


def username_from_url(url):
    if not url:
        return ""
    parsed = parse.urlparse(url)
    parts = [part for part in parsed.path.split("/") if part]
    for part in parts:
        if part.startswith("@"):
            return part[1:]
    return ""


def normalize_username(value):
    value = str(value or "").strip().strip("/")
    if value.startswith("@"):
        value = value[1:]
    return value


def video_url(username, post_id):
    if not username or not post_id:
        return ""
    return f"https://www.tiktok.com/@{username}/video/{post_id}"


def first(*values):
    for value in values:
        if value not in ("", None, "<nil>"):
            return value
    return ""


def number(value):
    try:
        if value in ("", None):
            return 0
        return int(float(value))
    except (TypeError, ValueError):
        return 0


def trim(value, max_len):
    value = str(value or "").strip()
    return value[:max_len]


def fail(message):
    print(message, file=sys.stderr)
    raise SystemExit(1)


if __name__ == "__main__":
    main()
