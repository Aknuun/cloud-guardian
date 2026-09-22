# ساخت توکن کلادفلر — فقط ۱ بار تیک بزن

### ۱. برو به:
https://dash.cloudflare.com/profile/api-tokens → `Create Custom Token`

### ۲. این ۹ تا رو تیک بزن:

**اجباری (۴ تا):**
- [x] **Account** → `Workers Scripts` → **Edit**
- [x] **Account** → `Workers KV Storage` → **Edit**
- [x] **Zone** → `DNS` → **Edit**
- [x] **Zone** → `Zone Settings` → **Edit**

**اختیاری (۵ تا):**
- [x] **Zone** → `Cache Purge` → **Purge**
- [x] **Zone** → `Email Routing Rules` → **Edit**
- [x] **Account** → `Email Routing Addresses` → **Edit**
- [x] **Zone** → `Analytics` → **Read**
- [x] **Account** → `Account Analytics` → **Read**

> Account Resources = **All accounts** · Zone Resources = **All zones**

### ۳. بزن:
`Continue → Create Token` → توکن `cfut_...` رو کپی کن و به اسکریپت بده

> بقیه خودکار انجام میشه.
