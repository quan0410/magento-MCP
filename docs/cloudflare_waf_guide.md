# Hướng Dẫn Vượt Cloudflare WAF & Zero-Friction Onboarding Cho Magento

Tài liệu này giải thích cơ chế vượt Cloudflare Bot Management và hướng dẫn cấu hình WAF Bypass cho các store Magento kết nối với DatumBridge MCP.

---

## 1. Cơ Chế Mặc Định: Giả Lập Trình Duyệt Tầng TLS (Zero-Config)

Hệ thống DatumBridge Magento MCP và Backend Integrations đã được tích hợp bộ máy HTTP Client mô phỏng trình duyệt (`tls-client` với profile `Chrome 131`).

### Nguyên lý hoạt động:
* **Bộ bắt tay TLS (JA3/JA4 Fingerprint):** Giả lập chính xác tập Cipher Suites, ALPN (`h2, http/1.1`), và TLS Extensions của Google Chrome trên macOS/Windows.
* **Browser Headers:** Tự động gửi kèm đầy đủ các header client-hints (`sec-ch-ua`, `sec-fetch-*`, `user-agent` Chrome).
* **Kết quả:** **95%+ các store Magento nằm sau Cloudflare (gói Free, Pro, Business) sẽ thông luồng tự động** mà merchant không cần thao tác gì.

---

## 2. Cơ Chế Dự Phòng: WAF Custom Secret Header (Dành Cho Store Bật Super Bot Fight Mode)

Nếu merchant bật chế độ bảo mật cực kỳ khắt khe của Cloudflare (*Super Bot Fight Mode* hoặc *Under Attack Mode*) khiến các request API bị chặn `HTTP 403`, merchant chỉ cần tạo 1 Custom WAF Rule đơn giản trên Cloudflare theo hướng dẫn dưới đây.

### Bước 1: Cấu hình Secret Token
Trong cấu hình kết nối của merchant hoặc biến môi trường:
* Header Name: `MAGENTO_CUSTOM_HEADER_NAME` (ví dụ: `X-DatumBridge-Key`)
* Header Value: `MAGENTO_CUSTOM_HEADER_VALUE` (ví dụ: chuỗi bí mật ngẫu nhiên `db_sec_xxxxxxxxxxxx`)

### Bước 2: Tạo WAF Custom Rule trên Cloudflare Dashboard của Merchant
1. Đăng nhập vào **Cloudflare Dashboard** $\rightarrow$ Chọn tên miền Magento.
2. Điều hướng tới: **Security** $\rightarrow$ **WAF** $\rightarrow$ **Custom rules** $\rightarrow$ Bấm **Create rule**.
3. Điền thông tin rule:
   * **Rule name:** `Bypass DatumBridge MCP Connector`
   * **Expression Editor (Chuyển sang chế độ Edit expression):**
     ```text
     (http.request.uri.path contains "/rest/" and http.request.headers["x-datumbridge-key"] eq "db_sec_xxxxxxxxxxxx")
     ```
   * **Action:** Chọn **Skip**
   * **WAF components to skip:**
     * ✅ **All remaining custom rules**
     * ✅ **All managed rules (WAF Managed Rules)**
     * ✅ **Super Bot Fight Mode / Bot Management**
     * ✅ **Browser Integrity Check**
4. Bấm **Deploy**.

---

## 3. Mã Lỗi & Xử Lý

| Mã lỗi | Nguyên nhân | Cách khắc phục |
| :--- | :--- | :--- |
| `HTTP 200` | Thành công | Không cần thao tác |
| `HTTP 401` | Sai Access Token hoặc OAuth Signature | Kiểm tra lại Access Token / Consumer Key / Consumer Secret trên Magento Admin |
| `HTTP 403` (kèm `Cloudflare challenge/WAF blocked request`) | Store bật Super Bot Fight Mode chặn tầng IP/ASN | Cung cấp Custom Header Secret và hướng dẫn merchant tạo Rule Skip WAF theo Mục 2 |
