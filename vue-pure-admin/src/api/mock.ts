import { http } from "@/utils/http";

type Result = {
  code: number;
  message: string;
  data: any;
};

/** 地图数据 */
export const mapJson = (params?: object) => {
  return http.request<Result>("get", "/get-map-info", { params });
};

/** 文件上传 */
export const formUpload = data => {
  return http.request<Result>("post", "/upload/images", { data }, {
    headers: {
      "Content-Type": "multipart/form-data"
    }
  });
};
