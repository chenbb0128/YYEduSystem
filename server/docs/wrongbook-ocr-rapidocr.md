# 豆芽成长助手错题集免费 OCR 接入说明

推荐先使用本地 RapidOCR。它不按调用次数收费，也不需要云厂商密钥；代价是需要在 API 服务器旁边跑一个轻量 OCR 服务。

## 1. 安装本地 OCR 依赖

在后端项目目录执行：

```powershell
cd D:\xilin\server
python -m pip install -r .\scripts\requirements-rapidocr.txt
```

如果服务器没有独立 GPU，CPU 也可以跑，只是识别速度会慢一些。第一版错题集仍保留老师校对流程，所以速度和准确率都不用一开始追到极致。

## 2. 启动 RapidOCR 服务

```powershell
cd D:\xilin\server
python .\scripts\rapidocr_service.py --host 127.0.0.1 --port 9009
```

启动成功后会监听：

```text
http://127.0.0.1:9009/ocr
```

健康检查：

```powershell
curl http://127.0.0.1:9009/healthz
```

## 3. 开启后端 OCR 配置

配置文件：

```yaml
ocr:
  enabled: true
  provider: rapidocr
  endpoint: "http://127.0.0.1:9009/ocr"
  action: rapidocr
  timeout: 10s
  max_image_bytes: 5242880
```

或者环境变量：

```env
TUOGUAN_SYSTEM_OCR_ENABLED=true
TUOGUAN_SYSTEM_OCR_PROVIDER=rapidocr
TUOGUAN_SYSTEM_OCR_ENDPOINT=http://127.0.0.1:9009/ocr
TUOGUAN_SYSTEM_OCR_ACTION=rapidocr
TUOGUAN_SYSTEM_OCR_TIMEOUT=10s
TUOGUAN_SYSTEM_OCR_MAX_IMAGE_BYTES=5242880
```

然后重启后端 API。

## 4. 使用流程

老师端仍然走现有错题集页面：

1. 选择学生和科目。
2. 上传学生作业照片。
3. 点击提取题目。
4. 后端读取上传图片，调用本地 RapidOCR，拆成题目候选。
5. 老师勾选错题并校对。
6. 保存进学生错题集，可继续生成复习卷。

## 5. 注意事项

- 免费 OCR 对清晰印刷题、整洁手写效果更好；对潦草手写、复杂数学公式、竖式计算，仍建议老师校对。
- 小程序和管理端不需要知道 RapidOCR 地址，仍然只访问自己的后端接口。
- 如果 OCR 服务没启动，后端会返回待校对占位题目，不会影响老师手动录入。
- 如果正式部署在 Docker 网络里，可以把 endpoint 改成 `http://rapidocr:9009/ocr`。
