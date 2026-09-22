# ساخت توکن کلادفلر — فقط ۱ بار تیک بزن

### ۱. برو به:
https://dash.cloudflare.com/profile/api-tokens → `Create Custom Token`

### ۲. این ۹ تا رو تیک بزن (۴ اجباری + ۵ اختیاری):

- [x] **Account** → `Workers Scripts` → **Edit**
- [x] **Account** → `Workers KV Storage` → **Edit**
- [x] **Zone** → `DNS` → **Edit**
- [x] **Zone** → `Zone Settings` → **Edit**
- [x] **Zone** → `Cache Purge` → **Purge** _(اختیاری)_
- [x] **Zone** → `Email Routing Rules` → **Edit** _(اختیاری)_
- [x] **Account** → `Email Routing Addresses` → **Edit** _(اختیاری)_
- [x] **Zone** → `Analytics` → **Read** _(اختیاری)_
- [x] **Account** → `Account Analytics` → **Read** _(اختیاری)_

> Account Resources = **All accounts** · Zone Resources = **All zones**

### ۳. بزن:
`Continue → Create Token` → توکن `cfut_...` رو کپی کن و به اسکریپت بده

> بقیه خودکار انجام میشه.
