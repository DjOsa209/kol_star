package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"
)

// IP stage notifications use the same Feishu application message transport as
// project import notifications. The sender is injectable so tests never contact
// a real tenant.
type ipStageMessageSender interface {
	sendToEmail(context.Context, string, string) error
}

type ipStageNotificationResult struct {
	Sent    int
	Skipped int
	Failed  int
}

type ipDirectoryRecipient struct {
	id         string
	employeeNo string
}

var ipStageRecipientKeys = map[string][]string{
	"submitted":          {"ipMarketing"},
	"ip_reviewed":        {"imc", "brand", "user", "media", "pr"},
	"marketing_reviewed": {"product"},
}

var ipStageNotificationTitles = map[string]string{
	"submitted":          "新的 IP 需求待初步反馈",
	"ip_reviewed":        "IP 组已完成初步反馈，请补充营销意见",
	"marketing_reviewed": "营销意见已补充，请查看 IP 需求",
}

// notifyIPStageAsync must only be called after a stage transition has committed.
// Delivery errors are logged and never roll back the user's successful submit.
func (a *app) notifyIPStageAsync(requestID int64, stage string) {
	cfg := a.Config()
	if !cfg.Feishu.ApplicationEnabled || strings.TrimSpace(cfg.Feishu.AppID) == "" || strings.TrimSpace(cfg.Feishu.AppSecret) == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		result, err := a.sendIPStageNotifications(ctx, cfg, requestID, stage, newFeishuClient(cfg.Feishu, nil))
		if err != nil {
			log.Printf("[ip-notification][request=%d][stage=%s] preparation failed (%T)", requestID, stage, err)
			return
		}
		log.Printf("[ip-notification][request=%d][stage=%s] sent=%d skipped=%d failed=%d", requestID, stage, result.Sent, result.Skipped, result.Failed)
	}()
}

func (a *app) sendIPStageNotifications(ctx context.Context, cfg Config, requestID int64, stage string, sender ipStageMessageSender) (ipStageNotificationResult, error) {
	var result ipStageNotificationResult
	if _, ok := ipStageRecipientKeys[stage]; !ok {
		return result, fmt.Errorf("unknown IP notification stage %q", stage)
	}
	if !cfg.Feishu.ApplicationEnabled || strings.TrimSpace(cfg.Feishu.AppID) == "" || strings.TrimSpace(cfg.Feishu.AppSecret) == "" {
		return result, nil
	}
	if sender == nil {
		return result, errors.New("IP notification sender is nil")
	}
	var projectName, briefJSON, submittedAt, ipReviewedAt string
	var creatorID int64
	if err := a.DB().QueryRowContext(ctx,
		`select project_name, coalesce(brief,''), created_by,
        coalesce(date_format(submitted_at,'%Y-%m-%d %H:%i:%s'),''),
        coalesce(date_format(ip_reviewed_at,'%Y-%m-%d %H:%i:%s'),'') from biz_ip_requests where id=?`, requestID,
	).Scan(&projectName, &briefJSON, &creatorID, &submittedAt, &ipReviewedAt); err != nil {
		return result, fmt.Errorf("read IP request for notification: %w", err)
	}
	var brief map[string]any
	if strings.TrimSpace(briefJSON) != "" {
		if err := json.Unmarshal([]byte(briefJSON), &brief); err != nil {
			return result, fmt.Errorf("decode IP request contacts: %w", err)
		}
	}
	ids := ipStageContactIDs(brief, creatorID, stage)
	directoryRecipients := ipStageDirectoryRecipients(brief, stage)
	directory := newUACDirectoryClient(cfg.SSO, nil)
	message := ipStageFeishuMessage(cfg, requestID, projectName, stage)
	if stage == "submitted" || stage == "ip_reviewed" {
		calendar, err := newWorkCalendar(cfg.WorkCalendar)
		if err != nil {
			return result, err
		}
		start, label, days := ipStageTime(submittedAt), "IP意见", 10
		if stage == "ip_reviewed" {
			start, label, days = ipStageTime(ipReviewedAt), "营销意见", 5
		}
		deadline := calendar.stageDeadline(label, days, start, time.Time{}, true, false, time.Now())
		message += fmt.Sprintf("\n反馈周期：%d个工作日（次日起算，含法定调休）", days)
		if deadline.DueAt != "" {
			message += "\n最晚反馈：" + ipStageTime(deadline.DueAt).Format("2006-01-02 15:04") + "（北京时间）"
		} else {
			message += "\n" + deadline.Warning
		}
	}
	sentEmails := map[string]bool{}
	deliver := func(email string) {
		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" || sentEmails[email] {
			result.Skipped++
			return
		}
		sentEmails[email] = true
		if err := sender.sendToEmail(ctx, email, message); err != nil {
			result.Failed++
			log.Printf("[ip-notification][request=%d][stage=%s] one recipient delivery failed (%T)", requestID, stage, err)
			return
		}
		result.Sent++
	}
	if len(directoryRecipients) > 0 {
		for _, recipient := range directoryRecipients {
			if !directory.configured() {
				result.Skipped++
				continue
			}
			person, err := directory.byEmployeeNo(ctx, recipient.employeeNo)
			if err != nil || !person.Notifiable || person.ID != recipient.id {
				result.Skipped++
				continue
			}
			deliver(person.email)
		}
	}
	for _, id := range ids {
		var email, provider string
		var status int
		err := a.DB().QueryRowContext(ctx,
			"select coalesce(email,''), status, auth_provider from sys_users where id=?", id,
		).Scan(&email, &status, &provider)
		if errors.Is(err, sql.ErrNoRows) {
			result.Skipped++
			continue
		}
		if err != nil {
			return result, fmt.Errorf("resolve IP stage recipient: %w", err)
		}
		// Stage contacts must be selected from SSO colleagues. The creator is
		// included after marketing completion even if they use a local account.
		if status != 1 || (id != creatorID && provider != "uac") {
			result.Skipped++
			continue
		}
		if provider == "uac" && directory.configured() {
			var employeeNo, subject string
			if err := a.DB().QueryRowContext(ctx,
				"select employee_no, coalesce(external_subject,'') from sys_users where id=?", id,
			).Scan(&employeeNo, &subject); err != nil {
				return result, fmt.Errorf("resolve cached SSO recipient: %w", err)
			}
			person, err := directory.byEmployeeNo(ctx, employeeNo)
			if err != nil || !person.Notifiable || (subject != person.uid && subject != person.EmployeeNo) {
				result.Skipped++
				continue
			}
			deliver(person.email)
			continue
		}
		deliver(email)
	}
	return result, nil
}

func ipStageContactIDs(brief map[string]any, creatorID int64, stage string) []int64 {
	contacts, _ := brief["contactsByStage"].(map[string]any)
	seen := map[int64]bool{}
	ids := []int64{}
	for _, key := range ipStageRecipientKeys[stage] {
		contact, _ := contacts[key].(map[string]any)
		id := anyInt64(contact["id"])
		if id > 0 && !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	if stage == "marketing_reviewed" && creatorID > 0 && !seen[creatorID] {
		ids = append(ids, creatorID)
	}
	return ids
}

func ipStageDirectoryRecipients(brief map[string]any, stage string) []ipDirectoryRecipient {
	contacts, _ := brief["contactsByStage"].(map[string]any)
	seen := map[string]bool{}
	recipients := []ipDirectoryRecipient{}
	for _, key := range ipStageRecipientKeys[stage] {
		contact, _ := contacts[key].(map[string]any)
		employeeNo := ipMapText(contact, "employeeNo")
		id := ipMapText(contact, "id")
		// Free text and numeric local IDs cannot prove which directory user
		// was selected, so they never trigger a remote employee lookup.
		if employeeNo != "" && strings.HasPrefix(id, "uac:") && len(id) > len("uac:") && !seen[employeeNo] {
			seen[employeeNo] = true
			recipients = append(recipients, ipDirectoryRecipient{id: id, employeeNo: employeeNo})
		}
	}
	return recipients
}

func ipStageFeishuMessage(cfg Config, requestID int64, projectName, stage string) string {
	lines := []string{"XMP IP 需求提醒", ipStageNotificationTitles[stage], "项目：" + strings.TrimSpace(projectName)}
	frontendURL := strings.TrimSpace(cfg.Feishu.FrontendURL)
	if frontendURL == "" {
		frontendURL = strings.TrimSpace(cfg.SSO.FrontendURL)
	}
	if parsed, err := url.Parse(frontendURL); err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" {
		path := "/business/ip/requests"
		if stage == "submitted" {
			path = "/business/ip/feedback"
		} else if stage == "ip_reviewed" {
			path = "/business/ip/marketing"
		}
		parsed.Fragment = fmt.Sprintf("%s?id=%d", path, requestID)
		lines = append(lines, "查看需求："+parsed.String())
	}
	return strings.Join(lines, "\n")
}
