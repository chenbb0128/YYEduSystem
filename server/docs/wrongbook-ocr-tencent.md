# 豆芽成长助手错题集腾讯云 OCR 接入说明

这是备用的云 OCR 方案。免费方案优先看 `docs/wrongbook-ocr-rapidocr.md`。

错题集拍照提题由后端调用腾讯云 OCR，小程序和管理端只负责上传图片并调用现有接口：

```http
POST /api/v1/wrong-questions/extract
```

请求体保持不变：

```json
{
  "image_url": "/uploads/homework/xxx.jpg",
  "source_text": "",
  "subject": "数学"
}
```

## 腾讯云配置

在服务端配置文件或环境变量中启用：

```yaml
ocr:
  enabled: true
  provider: tencent
  secret_id: "腾讯云 SecretId"
  secret_key: "腾讯云 SecretKey"
  region: ap-guangzhou
  endpoint: "https://ocr.tencentcloudapi.com/"
  action: GeneralHandwritingOCR
  timeout: 10s
  max_image_bytes: 5242880
```

等价环境变量：

```env
TUOGUAN_SYSTEM_OCR_ENABLED=true
TUOGUAN_SYSTEM_OCR_PROVIDER=tencent
TUOGUAN_SYSTEM_OCR_SECRET_ID=腾讯云 SecretId
TUOGUAN_SYSTEM_OCR_SECRET_KEY=腾讯云 SecretKey
TUOGUAN_SYSTEM_OCR_REGION=ap-guangzhou
TUOGUAN_SYSTEM_OCR_ENDPOINT=https://ocr.tencentcloudapi.com/
TUOGUAN_SYSTEM_OCR_ACTION=GeneralHandwritingOCR
TUOGUAN_SYSTEM_OCR_TIMEOUT=10s
TUOGUAN_SYSTEM_OCR_MAX_IMAGE_BYTES=5242880
```

## 行为说明

- 如果 `source_text` 有内容，后端优先按文字拆题，不调用 OCR。
- 如果 `source_text` 为空且 `image_url` 有内容，后端会读取已上传图片，转 base64 后调用腾讯云 OCR。
- 如果 OCR 未配置、图片读取失败或腾讯云临时异常，接口会返回 `warning` 和待校对题目占位，不会阻断老师手动录入错题。
- 第一版使用腾讯云 `GeneralHandwritingOCR` 提取文字，再由服务端规则拆成题目候选；后续可以在同一接口下切换到更细的题目识别能力。
