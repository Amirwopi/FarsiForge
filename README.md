# 🔨 FarsiForge — فارسی‌ساز بازی

<p align="center">
  <strong>ابزار حرفه‌ای فارسی‌سازی و بومی‌سازی بازی‌های ویدیویی</strong>
</p>

<p align="center" dir="rtl">
  استخراج متن • ترجمه • پردازش راست‌به‌چپ • تزریق • ساخت نصاب
</p>

---

## معرفی

FarsiForge یک ابزار متن‌باز برای ساده‌تر کردن فرآیند فارسی‌سازی بازی‌های ویدیویی است. این ابزار به‌صورت خودکار موتور بازی‌سازی را تشخیص می‌دهد، متن‌ها و دیالوگ‌های بازی را استخراج می‌کند، و پس از ترجمه، ترجمه‌ها را با پردازش صحیح راست‌به‌چپ و شکل‌دهی حروف فارسی در بازی تزریق می‌کند. در نهایت یک نصاب/لانچر (`installer.exe`) می‌سازد که می‌توانید آن را برای دیگران منتشر کنید.

## موتورهای پشتیبانی‌شده

| موتور | تشخیص | استخراج | تزریق | نسخه |
|-------|:-----:|:-------:|:-----:|------|
| Unity (Mono/IL2CPP) | ✅ | ✅ UnityPy | ✅ | همه نسخه‌ها |
| Unreal Engine | ✅ | ✅ UnrealLocres | ✅ | UE4/UE5 |
| Godot | ✅ | ✅ gdre_tools | ⚠️ | 3.x/4.x |
| RPG Maker MV/MZ | ✅ | ✅ بومی | ✅ | MV/MZ |
| GameMaker | ✅ | ⚠️ UndertaleModTool | ⚠️ | GMS1/2 |
| Ren'Py | ✅ | ✅ بومی | ✅ | — |
| Source / GoldSrc | ✅ | ✅ بومی | ✅ | — |
| Adobe AIR | ✅ | ✅ بومی | ✅ | — |
| Generic Text Files | — | ✅ بومی | ✅ | — |

## امکانات

- **تشخیص خودکار موتور بازی** با بررسی ساختار پوشه و فایل‌های مشخصه
- **استخراج متن و دیالوگ** از انواع فرمت‌های بازی
- **ویرایشگر ترجمه داخلی** با پیش‌نمایش زنده
- **خروجی/ایمپورت Excel (XLSX) و CSV** برای ترجمه تیمی
- **پردازش حروف فارسی**: شکل‌دهی (Reshape)، مرتب‌سازی راست‌به‌چپ (BiDi)، اصلاح ی/ک فارسی، اعداد فارسی
- **ساخت نصاب/لانچر**: یک فایل `installer.exe` مستقل با منوی فارسی
- **پشتیبانی از فونت**: تزریق خودکار فونت فارسی (Vazirmatn)
- **رابط کاربری وب** با طراحی مدرن و راست‌به‌چپ

## نصب و اجرا

### پیش‌نیازها

- [Go 1.21+](https://go.dev/dl/)
- [Python 3.10+](https://www.python.org/) با پکیج‌های:
  ```bash
  pip install UnityPy arabic-reshaper python-bidi fonttools openpyxl
  ```

### ساخت از سورس

```bash
git clone https://github.com/Amirwopi/FarsiForge.git
cd FarsiForge
go build -o bin/farsiforge.exe ./cmd/farsiforge
```

### اجرا

```bash
bin/farsiforge.exe
```

برنامه در مرورگر باز می‌شود: `http://127.0.0.1:7842`

## مراحل استفاده

۱. **انتخاب بازی** — مسیر پوشه بازی را وارد کنید
۲. **تشخیص موتور** — موتور و نسخه بازی‌سازی به‌صورت خودکار شناسایی می‌شود
۳. **استخراج متن** — متن‌های قابل ترجمه استخراج می‌شوند
۴. **ترجمه** — متن‌ها را در ویرایشگر داخلی یا Excel ترجمه کنید
۵. **تزریق** — ترجمه‌ها با پردازش راست‌به‌چپ در بازی جایگزین می‌شوند
۶. **ساخت نصاب** — فایل `installer.exe` برای انتشار ساخته می‌شود

## ابزارهای مورد استفاده

FarsiForge از ابزارهای متن‌باز زیر استفاده می‌کند:

| ابزار | کاربرد |
|-------|--------|
| [UnityPy](https://github.com/K0lb3/UnityPy) | استخراج و تزریق متن از Unity |
| [UnrealLocres](https://github.com/amrshaheen/UnrealLocres) | استخراج .locres اونریل |
| [gdre_tools](https://github.com/bruvzg/gdsdecomp) | استخراج PCK گودو |
| [UndertaleModTool](https://github.com/UnderminersTeam/UndertaleModTool) | استخراج data.win گیدمیکر |
| [Vazirmatn](https://github.com/rastikerdar/vazirmatn) | فونت فارسی |
| [Detect It Easy](https://github.com/horsicq/Detect-It-Easy) | تحلیل باینری |
| [repak](https://github.com/trumank/repak) | باز/بسته کردن .pak اونریل |

## ساختار پروژه

```
FarsiForge/
├── cmd/
│   ├── farsiforge/          # برنامه اصلی (سرور وب + رابط کاربری)
│   │   ├── main.go
│   │   └── web/             # رابط کاربری (HTML/CSS/JS)
│   └── farsiforge-installer/  # قالب نصاب/لانچر
│       └── main.go
├── pkg/
│   ├── persian/             # پردازش متن فارسی (Reshape, BiDi, ارقام)
│   ├── detection/           # تشخیص موتور بازی
│   ├── extract/             # استخراج متن از بازی‌ها
│   ├── inject/              # تزریق ترجمه به بازی‌ها
│   ├── exchange/            # خروجی/ایمپورت XLSX/CSV
│   ├── installer/           # ساخت نصاب
│   ├── project/             # مدیریت پروژه
│   └── tools/               # کشف و مدیریت ابزارها
└── web/                     # فایل‌های رابط کاربری
```

## مشارکت

این پروژه متن‌باز است و از مشارکت استقبال می‌کند. اگر برنامه‌نویس، مترجم، Modder یا علاقه‌مند به فارسی‌سازی بازی‌ها هستید، می‌توانید در توسعه مشارکت کنید.

## لایسنس

MIT License
