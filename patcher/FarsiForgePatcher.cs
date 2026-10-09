// FarsiForge Patcher — single-file C# patcher (GUI + CLI) for Persian game localization.
//
// Engine: .NET Framework 4.x (any Windows 7+) | C# 5 | WinForms + CLI
//
// FFP1 binary patch format (generalization of the Machine Party "MPP1" format):
//
//   Header:
//     magic "FFP1" (4 ASCII bytes)
//     u32  version = 1
//     metadata: 8 strings, each (u32 len + UTF-8 bytes):
//       patch_name, game_name, game_exe, engine, patch_version,
//       author, description, created_at (ISO 8601)
//     u32 target_count
//     i64 blob_size
//   Targets (repeated target_count times):
//     u32  path_len + path UTF-8   (game-RELATIVE target file path,
//                                   e.g. "SupermarketTogether_Data/resources.assets")
//     u8   mode                    (0 = rebuild from records, 1 = replace = single payload record)
//     i64  final_size              (uncompressed size of the final target file)
//     32 bytes sha256              (of the final target file)
//     u32  record_count
//     Records (repeated record_count times):
//       u32  path_len + path UTF-8 (logical name, for logging)
//       u8   src                   (0 = copy-from-base, 1 = embedded payload)
//       if src==0:
//         u32  base_len + base_path UTF-8 (game-relative base file)
//         i64  offset
//         i64  size
//       if src==1:
//         i64  payload_ofs (blob-relative)
//         i64  z_size
//         i64  raw_size
//       16 bytes md5              (of the record's uncompressed content)
//   Blob: raw DEFLATE streams (System.IO.Compression DeflateStream-compatible)
//
// All integers little-endian (BinaryReader/Writer default). Everything UTF-8.
//
// Build (GUI, ships to users):
//   csc /nologo /target:winexe /platform:anycpu /out:FarsiForgePatcher.exe ^
//       /r:System.Windows.Forms.dll /r:System.Drawing.dll FarsiForgePatcher.cs
// Build (CLI, for scripted/testing use):
//   csc /nologo /target:exe /define:CLI /out:FarsiForgePatcherCli.exe ^
//       /r:System.Windows.Forms.dll /r:System.Drawing.dll FarsiForgePatcher.cs
// Run (GUI):  put FarsiForgePatcher.exe + patch.ffpatch next to the game, run, click نصب.
// Run (CLI):  FarsiForgePatcherCli.exe <patch.ffpatch> [gameDir]
//             FarsiForgePatcherCli.exe --info <patch.ffpatch>
//             FarsiForgePatcherCli.exe --uninstall <patch.ffpatch> [gameDir]

using System;
using System.Collections.Generic;
using System.Drawing;
using System.IO;
using System.IO.Compression;
using System.Security.Cryptography;
using System.Text;
using System.Threading;
using System.Windows.Forms;

namespace FarsiForgePatcher
{
    // ---- core patch engine (shared by GUI and CLI) ----

    public class Record
    {
        public string Path;        // logical name (for logging)
        public byte Src;            // 0 = copy-from-base, 1 = embedded payload
        // src == 0:
        public string BasePath;    // game-relative base file
        public long Offset;
        public long Size;
        // src == 1:
        public long PayloadOfs;    // blob-relative
        public long ZSize;         // compressed size
        public long RawSize;       // uncompressed size
        public byte[] Md5;         // 16 bytes, of the record's uncompressed content
    }

    public class Target
    {
        public string Path;        // game-relative target file
        public byte Mode;          // 0 = rebuild from records, 1 = replace (single payload record)
        public long FinalSize;     // uncompressed size of the final target file
        public byte[] Sha256;      // 32 bytes, of the final target file
        public List<Record> Records = new List<Record>();
    }

    public class PatchInfo
    {
        public uint Version;
        public string PatchName;
        public string GameName;
        public string GameExe;
        public string Engine;
        public string PatchVersion;
        public string Author;
        public string Description;
        public string CreatedAt;
        public List<Target> Targets = new List<Target>();
        public long BlobSize;
        public long BlobPos;
        public long TotalSize;     // sum of all final_size, for progress
    }

    public static class PatchJob
    {
        // ---- header reading ----

        public static PatchInfo ReadHeader(BinaryReader r)
        {
            if (Encoding.ASCII.GetString(r.ReadBytes(4)) != "FFP1")
                throw new Exception("not a valid FFP1 patch file");
            uint ver = r.ReadUInt32();
            if (ver != 1) throw new Exception("unsupported patch version " + ver);
            PatchInfo info = new PatchInfo();
            info.Version = ver;
            info.PatchName    = ReadString(r);
            info.GameName     = ReadString(r);
            info.GameExe      = ReadString(r);
            info.Engine       = ReadString(r);
            info.PatchVersion = ReadString(r);
            info.Author       = ReadString(r);
            info.Description  = ReadString(r);
            info.CreatedAt    = ReadString(r);

            uint targetCount = r.ReadUInt32();
            info.BlobSize = r.ReadInt64();

            info.TotalSize = 0;
            for (uint i = 0; i < targetCount; i++)
            {
                Target t = new Target();
                t.Path = ReadString(r);
                t.Mode = r.ReadByte();
                t.FinalSize = r.ReadInt64();
                t.Sha256 = r.ReadBytes(32);
                uint recCount = r.ReadUInt32();
                for (uint j = 0; j < recCount; j++)
                {
                    Record rec = new Record();
                    rec.Path = ReadString(r);
                    rec.Src = r.ReadByte();
                    if (rec.Src == 0)
                    {
                        rec.BasePath = ReadString(r);
                        rec.Offset = r.ReadInt64();
                        rec.Size = r.ReadInt64();
                    }
                    else
                    {
                        rec.PayloadOfs = r.ReadInt64();
                        rec.ZSize = r.ReadInt64();
                        rec.RawSize = r.ReadInt64();
                    }
                    rec.Md5 = r.ReadBytes(16);
                    t.Records.Add(rec);
                }
                info.Targets.Add(t);
                info.TotalSize += t.FinalSize;
            }
            info.BlobPos = r.BaseStream.Position;
            return info;
        }

        // ---- apply ----

        // Apply the whole patch to a game folder. Returns list of swapped target paths.
        // On failure, rolls back already-swapped targets and deletes .ffnew temps.
        public static void ApplyAll(PatchInfo info, string patchPath, string gameDir,
            Action<long, long> progress, Action<string> log)
        {
            List<string> swapped = new List<string>(); // game-relative paths already swapped
            List<string> temps = new List<string>();   // .ffnew paths created
            long bytesDone = 0;
            long bytesTotal = info.TotalSize;

            try
            {
                foreach (Target t in info.Targets)
                {
                    string absTarget = LongPath(Path.Combine(gameDir, t.Path));
                    string ffnew = absTarget + ".ffnew";
                    string ffbak = absTarget + ".ffbak";

                    EnsureParentDir(ffnew);
                    if (File.Exists(ffnew)) File.Delete(ffnew);

                    if (log != null) log("در حال اعمال فایل‌ها: " + t.Path);
                    ApplyTarget(info, t, patchPath, gameDir, ffnew, ref bytesDone, bytesTotal, progress, log);

                    // verify final sha256 of .ffnew
                    if (log != null) log("بررسی صحت فایل‌ها: " + t.Path);
                    byte[] gotSha;
                    using (FileStream vf = File.OpenRead(ffnew))
                    using (SHA256 sha = SHA256.Create())
                        gotSha = sha.ComputeHash(vf);
                    if (!BytesEq(gotSha, t.Sha256))
                        throw new Exception("sha256 mismatch on target " + t.Path + ": got " + Hex(gotSha));

                    // swap: original -> .ffbak, .ffnew -> target
                    if (log != null) log("در حال پشتیبان‌گیری: " + t.Path);
                    if (File.Exists(ffbak)) File.Delete(ffbak);
                    if (File.Exists(absTarget)) File.Move(absTarget, ffbak);
                    File.Move(ffnew, absTarget);

                    swapped.Add(t.Path);
                    if (log != null) log("نصب شد: " + t.Path);
                }
                if (progress != null) progress(bytesTotal, bytesTotal);
            }
            catch (Exception ex)
            {
                // rollback
                if (log != null) log("خطا: در حال بازگردانی تغییرات…");
                foreach (string rel in swapped)
                {
                    try
                    {
                        string abs = LongPath(Path.Combine(gameDir, rel));
                        string ffbak = abs + ".ffbak";
                        if (File.Exists(ffbak))
                        {
                            if (File.Exists(abs)) File.Delete(abs);
                            File.Move(ffbak, abs);
                        }
                    }
                    catch { /* best-effort */ }
                }
                foreach (string tmp in temps)
                {
                    try { if (File.Exists(tmp)) File.Delete(tmp); } catch { }
                }
                throw new Exception("rollback: " + ex.Message, ex);
            }
        }

        // Write a single target's .ffnew by streaming its records.
        private static void ApplyTarget(PatchInfo info, Target t, string patchPath, string gameDir,
            string outPath, ref long bytesDone, long bytesTotal,
            Action<long, long> progress, Action<string> log)
        {
            using (FileStream pf = File.OpenRead(patchPath))
            using (FileStream of = File.Open(outPath, FileMode.Create, FileAccess.Write, FileShare.None))
            {
                byte[] buf = new byte[1024 * 1024];
                long written = 0;
                foreach (Record rec in t.Records)
                {
                    long copied = 0;
                    if (rec.Src == 0)
                    {
                        string baseAbs = LongPath(Path.Combine(gameDir, rec.BasePath));
                        if (!File.Exists(baseAbs))
                            throw new Exception("base file missing: " + rec.BasePath);
                        using (FileStream bf = File.OpenRead(baseAbs))
                        using (MD5 md5 = MD5.Create())
                        {
                            bf.Seek(rec.Offset, SeekOrigin.Begin);
                            while (copied < rec.Size)
                            {
                                int chunk = (int)Math.Min(buf.Length, rec.Size - copied);
                                int got = bf.Read(buf, 0, chunk);
                                if (got <= 0) throw new Exception("unexpected EOF in base file: " + rec.BasePath);
                                of.Write(buf, 0, got);
                                md5.TransformBlock(buf, 0, got, null, 0);
                                copied += got;
                                written += got;
                                bytesDone += got;
                                if (progress != null) progress(bytesDone, bytesTotal);
                            }
                            md5.TransformFinalBlock(buf, 0, 0);
                            if (!BytesEq(md5.Hash, rec.Md5))
                                throw new Exception("base content md5 mismatch: " + rec.Path);
                        }
                    }
                    else
                    {
                        byte[] zdata = new byte[rec.ZSize];
                        pf.Seek(info.BlobPos + rec.PayloadOfs, SeekOrigin.Begin);
                        ReadFull(pf, zdata, (int)rec.ZSize);
                        byte[] data = Inflate(zdata, rec.RawSize);
                        byte[] got;
                        using (MD5 m2 = MD5.Create()) got = m2.ComputeHash(data);
                        if (!BytesEq(got, rec.Md5))
                            throw new Exception("payload md5 mismatch: " + rec.Path);
                        of.Write(data, 0, data.Length);
                        copied = data.Length;
                        written += copied;
                        bytesDone += copied;
                        if (progress != null) progress(bytesDone, bytesTotal);
                    }
                }
                if (written != t.FinalSize)
                    throw new Exception("size mismatch on target " + t.Path + ": wrote " + written + ", expected " + t.FinalSize);
            }
        }

        // ---- uninstall ----

        public static void UninstallAll(PatchInfo info, string gameDir, Action<string> log)
        {
            foreach (Target t in info.Targets)
            {
                string abs = LongPath(Path.Combine(gameDir, t.Path));
                string ffbak = abs + ".ffbak";
                if (File.Exists(ffbak))
                {
                    if (File.Exists(abs)) File.Delete(abs);
                    File.Move(ffbak, abs);
                    if (log != null) log("بازگردانی شد: " + t.Path);
                }
                else
                {
                    if (log != null) log("پشتیبانی برای این فایل یافت نشد: " + t.Path);
                }
            }
        }

        // ---- helpers ----

        private static string ReadString(BinaryReader r)
        {
            uint n = r.ReadUInt32();
            return Encoding.UTF8.GetString(r.ReadBytes((int)n));
        }

        public static string Hex(byte[] b)
        {
            StringBuilder sb = new StringBuilder(b.Length * 2);
            foreach (byte x in b) sb.Append(x.ToString("x2"));
            return sb.ToString();
        }

        private static void ReadFull(FileStream f, byte[] buf, int len)
        {
            int off = 0;
            while (off < len)
            {
                int got = f.Read(buf, off, len - off);
                if (got <= 0) throw new Exception("unexpected EOF");
                off += got;
            }
        }

        private static byte[] Inflate(byte[] src, long expected)
        {
            using (MemoryStream ms = new MemoryStream(src))
            using (DeflateStream ds = new DeflateStream(ms, CompressionMode.Decompress))
            using (MemoryStream om = new MemoryStream((int)expected))
            {
                byte[] b = new byte[1024 * 1024];
                for (; ; )
                {
                    int n = ds.Read(b, 0, b.Length);
                    if (n <= 0) break;
                    om.Write(b, 0, n);
                }
                if (om.Length != expected)
                    throw new Exception("inflate size mismatch: " + om.Length + " vs " + expected);
                return om.ToArray();
            }
        }

        public static bool BytesEq(byte[] a, byte[] b)
        {
            if (a == null || b == null || a.Length != b.Length) return false;
            for (int i = 0; i < a.Length; i++) if (a[i] != b[i]) return false;
            return true;
        }

        // Prefix long paths with \\?\ to bypass MAX_PATH limits.
        public static string LongPath(string p)
        {
            if (string.IsNullOrEmpty(p)) return p;
            string full = Path.GetFullPath(p);
            if (full.Length >= 260 && !full.StartsWith(@"\\?\"))
                return @"\\?\" + full;
            return full;
        }

        private static void EnsureParentDir(string file)
        {
            string dir = Path.GetDirectoryName(file);
            if (!string.IsNullOrEmpty(dir) && !Directory.Exists(dir))
                Directory.CreateDirectory(dir);
        }
    }

    // ---- GUI ----
#if !CLI
    public class MainForm : Form
    {
        private Label titleLabel;
        private Label subLabel;
        private Label statusLabel;
        private TextBox logBox;
        private ProgressBar bar;
        private Button browseBtn;
        private Button applyBtn;
        private Button uninstallBtn;
        private Button launchBtn;

        private string patchPath;
        private PatchInfo info;
        private string gameDir;

        // dark theme palette
        private static readonly Color Bg     = Color.FromArgb(0x14, 0x14, 0x1F);
        private static readonly Color Panel  = Color.FromArgb(0x1E, 0x1E, 0x2E);
        private static readonly Color Accent = Color.FromArgb(0x7C, 0x5C, 0xFF);
        private static readonly Color FgText = Color.FromArgb(0xEA, 0xEA, 0xF2);
        private static readonly Color Muted  = Color.FromArgb(0x9A, 0x9A, 0xB0);
        private static readonly Font BaseFont = new Font("Tahoma", 9.5f);

        public MainForm()
        {
            Text = "FarsiForge — نصب فارسی‌سازی";
            FormBorderStyle = FormBorderStyle.FixedDialog;
            MaximizeBox = false;
            MinimizeBox = false;
            StartPosition = FormStartPosition.CenterScreen;
            ClientSize = new Size(600, 420);
            RightToLeft = RightToLeft.Yes;
            RightToLeftLayout = true;
            BackColor = Bg;
            ForeColor = FgText;
            Font = BaseFont;

            BuildUi();

            try { LocatePatch(); }
            catch (Exception ex)
            {
                Log("خطا: " + ex.Message);
                statusLabel.Text = "بازی پیدا نشد — پوشه بازی را انتخاب کنید";
                applyBtn.Enabled = false;
                uninstallBtn.Enabled = false;
                launchBtn.Enabled = false;
            }
        }

        private void BuildUi()
        {
            // header panel
            Panel header = new Panel();
            header.BackColor = Panel;
            header.Location = new Point(0, 0);
            header.Size = new Size(600, 64);
            header.Dock = DockStyle.Top;
            Controls.Add(header);

            titleLabel = new Label();
            titleLabel.AutoSize = false;
            titleLabel.Location = new Point(16, 10);
            titleLabel.Size = new Size(568, 26);
            titleLabel.Font = new Font("Tahoma", 13f, FontStyle.Bold);
            titleLabel.ForeColor = FgText;
            titleLabel.BackColor = Panel;
            titleLabel.Text = "FarsiForge";
            titleLabel.TextAlign = ContentAlignment.MiddleLeft;
            header.Controls.Add(titleLabel);

            subLabel = new Label();
            subLabel.AutoSize = false;
            subLabel.Location = new Point(16, 36);
            subLabel.Size = new Size(568, 20);
            subLabel.Font = new Font("Tahoma", 9f);
            subLabel.ForeColor = Muted;
            subLabel.BackColor = Panel;
            subLabel.Text = "";
            subLabel.TextAlign = ContentAlignment.MiddleLeft;
            header.Controls.Add(subLabel);

            // status + progress
            statusLabel = new Label();
            statusLabel.Location = new Point(16, 74);
            statusLabel.Size = new Size(568, 20);
            statusLabel.Font = BaseFont;
            statusLabel.ForeColor = FgText;
            statusLabel.BackColor = Bg;
            statusLabel.Text = "آماده";
            statusLabel.TextAlign = ContentAlignment.MiddleLeft;
            Controls.Add(statusLabel);

            bar = new ProgressBar();
            bar.Location = new Point(16, 98);
            bar.Size = new Size(568, 18);
            bar.Style = ProgressBarStyle.Continuous;
            Controls.Add(bar);

            // log box
            logBox = new TextBox();
            logBox.Location = new Point(16, 124);
            logBox.Size = new Size(568, 232);
            logBox.Multiline = true;
            logBox.ReadOnly = true;
            logBox.ScrollBars = ScrollBars.Vertical;
            logBox.Font = new Font("Consolas", 9f);
            logBox.BackColor = Panel;
            logBox.ForeColor = FgText;
            logBox.BorderStyle = BorderStyle.FixedSingle;
            logBox.RightToLeft = RightToLeft.Yes;
            Controls.Add(logBox);

            // buttons row (RTL order: browse, apply, uninstall, launch)
            browseBtn = MakeBtn("انتخاب پوشه بازی", 462, OnBrowse);
            applyBtn = MakeBtn("نصب فارسی‌سازی", 354, OnApply);
            uninstallBtn = MakeBtn("حذف فارسی‌سازی", 246, OnUninstall);
            launchBtn = MakeBtn("اجرای بازی", 138, OnLaunch);
            Controls.Add(browseBtn);
            Controls.Add(applyBtn);
            Controls.Add(uninstallBtn);
            Controls.Add(launchBtn);
        }

        private Button MakeBtn(string text, int x, EventHandler handler)
        {
            Button b = new Button();
            b.Location = new Point(x, 366);
            b.Size = new Size(100, 34);
            b.Text = text;
            b.Font = BaseFont;
            b.FlatStyle = FlatStyle.Flat;
            b.FlatAppearance.BorderColor = Accent;
            b.FlatAppearance.BorderSize = 1;
            b.BackColor = Panel;
            b.ForeColor = FgText;
            b.TextAlign = ContentAlignment.MiddleCenter;
            b.Click += handler;
            return b;
        }

        private void LocatePatch()
        {
            string dir = AppDomain.CurrentDomain.BaseDirectory;
            string found = null;
            string[] all = Directory.GetFiles(dir, "*.ffpatch");
            if (all.Length == 1) found = all[0];
            else if (all.Length > 1)
            {
                using (OpenFileDialog d = new OpenFileDialog())
                {
                    d.Title = "انتخاب فایل پچ";
                    d.Filter = "FarsiForge patch (*.ffpatch)|*.ffpatch";
                    if (d.ShowDialog(this) == DialogResult.OK) found = d.FileName;
                }
            }
            if (found == null)
            {
                using (OpenFileDialog d = new OpenFileDialog())
                {
                    d.Title = "انتخاب فایل پچ";
                    d.Filter = "FarsiForge patch (*.ffpatch)|*.ffpatch";
                    if (d.ShowDialog(this) == DialogResult.OK) found = d.FileName;
                }
            }
            if (found == null) throw new Exception("فایل پچ (.ffpatch) پیدا نشد");

            patchPath = found;
            using (FileStream pf = File.OpenRead(patchPath))
            using (BinaryReader pr = new BinaryReader(pf))
                info = PatchJob.ReadHeader(pr);

            titleLabel.Text = string.IsNullOrEmpty(info.PatchName) ? "FarsiForge" : info.PatchName;
            subLabel.Text = info.GameName + "  —  نسخه " + info.PatchVersion +
                            (string.IsNullOrEmpty(info.Author) ? "" : "  —  " + info.Author);

            Log("پچ: " + Path.GetFileName(patchPath));
            Log("بازی: " + info.GameName + "  (" + info.Targets.Count + " فایل)");
            if (!string.IsNullOrEmpty(info.Description)) Log(info.Description);
            Log("");

            DetectGameDir();
        }

        private void DetectGameDir()
        {
            // 1. if game_exe exists next to the patcher exe, use exe dir
            string exeDir = AppDomain.CurrentDomain.BaseDirectory;
            if (!string.IsNullOrEmpty(info.GameExe))
            {
                string probe = Path.Combine(exeDir, info.GameExe);
                if (File.Exists(probe))
                {
                    gameDir = exeDir;
                    statusLabel.Text = "پوشه بازی: " + gameDir;
                    return;
                }
            }

            // 2. remember last folder
            string memFile = Path.Combine(exeDir, ".farsiforge_last_dir");
            if (File.Exists(memFile))
            {
                string last = File.ReadAllText(memFile, Encoding.UTF8).Trim();
                if (Directory.Exists(last) && GameExeExists(last))
                {
                    gameDir = last;
                    statusLabel.Text = "پوشه بازی: " + gameDir;
                    return;
                }
            }

            // 3. none yet — user must browse
            gameDir = null;
            statusLabel.Text = "بازی پیدا نشد — پوشه بازی را انتخاب کنید";
        }

        private bool GameExeExists(string dir)
        {
            if (string.IsNullOrEmpty(info.GameExe)) return false;
            return File.Exists(Path.Combine(dir, info.GameExe));
        }

        private void OnBrowse(object sender, EventArgs e)
        {
            using (FolderBrowserDialog d = new FolderBrowserDialog())
            {
                d.Description = "پوشه بازی را انتخاب کنید";
                d.ShowNewFolderButton = false;
                if (gameDir != null) d.SelectedPath = gameDir;
                if (d.ShowDialog(this) == DialogResult.OK)
                {
                    if (!GameExeExists(d.SelectedPath))
                    {
                        MessageBox.Show(this, "در این پوشه «" + (info.GameExe ?? "") + "» پیدا نشد.",
                            "FarsiForge", MessageBoxButtons.OK, MessageBoxIcon.Warning);
                        return;
                    }
                    gameDir = d.SelectedPath;
                    try { File.WriteAllText(Path.Combine(AppDomain.CurrentDomain.BaseDirectory,
                        ".farsiforge_last_dir"), gameDir, Encoding.UTF8); } catch { }
                    statusLabel.Text = "پوشه بازی: " + gameDir;
                }
            }
        }

        private void OnApply(object sender, EventArgs e)
        {
            if (info == null) return;
            if (gameDir == null)
            {
                MessageBox.Show(this, "ابتدا پوشه بازی را انتخاب کنید.", "FarsiForge",
                    MessageBoxButtons.OK, MessageBoxIcon.Information);
                return;
            }
            applyBtn.Enabled = false;
            uninstallBtn.Enabled = false;
            browseBtn.Enabled = false;
            launchBtn.Enabled = false;
            bar.Value = 0;
            statusLabel.ForeColor = FgText;
            Thread t = new Thread(new ThreadStart(ApplyGui));
            t.IsBackground = true;
            t.Start();
        }

        private void ApplyGui()
        {
            try
            {
                Log("");
                Log("شروع نصب فارسی‌سازی…");
                PatchJob.ApplyAll(info, patchPath, gameDir, Report, Log);
                SetProgress(100, "فارسی‌سازی با موفقیت نصب شد ✅", Accent);
                Log("");
                Log("نصب کامل شد. فایل‌های پشتیبان با پسوند .ffbak ذخیره شدند.");
                BeginInvoke(new Action(delegate { launchBtn.Enabled = true; }));
            }
            catch (Exception ex)
            {
                Log("");
                Log("خطا: " + ex.Message);
                BeginInvoke(new Action(delegate { statusLabel.Text = "نصب ناموفق بود"; }));
            }
            finally
            {
                BeginInvoke(new Action(delegate
                {
                    applyBtn.Enabled = true;
                    uninstallBtn.Enabled = true;
                    browseBtn.Enabled = true;
                }));
            }
        }

        private void OnUninstall(object sender, EventArgs e)
        {
            if (info == null) return;
            if (gameDir == null)
            {
                MessageBox.Show(this, "ابتدا پوشه بازی را انتخاب کنید.", "FarsiForge",
                    MessageBoxButtons.OK, MessageBoxIcon.Information);
                return;
            }
            DialogResult dr = MessageBox.Show(this, "آیا می‌خواهید فارسی‌سازی را حذف و فایل‌های اصلی را بازگردانی کنید؟",
                "حذف فارسی‌سازی", MessageBoxButtons.YesNo, MessageBoxIcon.Question);
            if (dr != DialogResult.Yes) return;

            try
            {
                PatchJob.UninstallAll(info, gameDir, Log);
                statusLabel.Text = "فارسی‌سازی حذف شد";
                statusLabel.ForeColor = FgText;
                Log("");
                Log("حذف فارسی‌سازی کامل شد.");
                launchBtn.Enabled = true;
            }
            catch (Exception ex)
            {
                Log("خطا: " + ex.Message);
                statusLabel.Text = "حذف ناموفق بود";
            }
        }

        private void OnLaunch(object sender, EventArgs e)
        {
            if (info == null || gameDir == null) return;
            try
            {
                string exe = Path.Combine(gameDir, info.GameExe);
                System.Diagnostics.Process.Start(new System.Diagnostics.ProcessStartInfo
                {
                    FileName = exe,
                    WorkingDirectory = gameDir,
                    UseShellExecute = true,
                });
            }
            catch (Exception ex)
            {
                MessageBox.Show(this, "اجرای بازی ناموفق بود: " + ex.Message, "FarsiForge",
                    MessageBoxButtons.OK, MessageBoxIcon.Warning);
            }
        }

        private void Report(long done, long total)
        {
            int pct = total > 0 ? (int)(done * 100 / total) : 0;
            if (pct > 100) pct = 100;
            BeginInvoke(new Action(delegate
            {
                bar.Value = pct;
                statusLabel.Text = (done / (1024 * 1024)) + " / " + (total / (1024 * 1024)) + " MB";
            }));
        }

        private void SetProgress(int pct, string s, Color color)
        {
            BeginInvoke(new Action(delegate
            {
                bar.Value = pct;
                statusLabel.Text = s;
                statusLabel.ForeColor = color;
            }));
        }

        private void Log(string s)
        {
            if (InvokeRequired)
            {
                BeginInvoke(new Action(delegate { Log(s); }));
                return;
            }
            logBox.AppendText(s + Environment.NewLine);
            logBox.SelectionStart = logBox.TextLength;
            logBox.ScrollToCaret();
        }
    }
#endif

    // ---- CLI ----
#if CLI
    public static class ConsoleMode
    {
        public static int Run(string[] args)
        {
            try
            {
                if (args.Length == 0)
                {
                    Usage();
                    return 2;
                }
                if (args[0] == "--info")
                {
                    if (args.Length < 2) { Usage(); return 2; }
                    return Info(args[1]);
                }
                if (args[0] == "--uninstall")
                {
                    if (args.Length < 2) { Usage(); return 2; }
                    string uGameDir = args.Length > 2 ? args[2] : null;
                    return Uninstall(args[1], uGameDir);
                }
                // default: apply
                string patchPath = args[0];
                string gameDir = args.Length > 1 ? args[1] : null;
                return Apply(patchPath, gameDir);
            }
            catch (Exception ex)
            {
                Console.WriteLine();
                Console.WriteLine("FAILED: " + ex.Message);
                return 1;
            }
        }

        private static void Usage()
        {
            Console.WriteLine("FarsiForge Patcher (CLI)");
            Console.WriteLine("  FarsiForgePatcherCli.exe <patch.ffpatch> [gameDir]");
            Console.WriteLine("  FarsiForgePatcherCli.exe --info <patch.ffpatch>");
            Console.WriteLine("  FarsiForgePatcherCli.exe --uninstall <patch.ffpatch> [gameDir]");
        }

        private static PatchInfo Load(string patchPath)
        {
            if (!File.Exists(patchPath)) throw new Exception("patch not found: " + patchPath);
            using (FileStream pf = File.OpenRead(patchPath))
            using (BinaryReader pr = new BinaryReader(pf))
                return PatchJob.ReadHeader(pr);
        }

        private static int Info(string patchPath)
        {
            PatchInfo info = Load(patchPath);
            Console.WriteLine("patch       : " + patchPath);
            Console.WriteLine("patch_name  : " + info.PatchName);
            Console.WriteLine("game_name   : " + info.GameName);
            Console.WriteLine("game_exe    : " + info.GameExe);
            Console.WriteLine("engine      : " + info.Engine);
            Console.WriteLine("patch_ver   : " + info.PatchVersion);
            Console.WriteLine("author      : " + info.Author);
            Console.WriteLine("description : " + info.Description);
            Console.WriteLine("created_at  : " + info.CreatedAt);
            Console.WriteLine("targets     : " + info.Targets.Count);
            foreach (Target t in info.Targets)
                Console.WriteLine("  - " + t.Path + "  (mode=" + t.Mode + ", size=" + t.FinalSize +
                    ", records=" + t.Records.Count + ")");
            return 0;
        }

        private static int Apply(string patchPath, string gameDir)
        {
            PatchInfo info = Load(patchPath);
            Console.WriteLine("patch : " + patchPath);
            Console.WriteLine("game  : " + info.GameName + " (" + info.Targets.Count + " files)");
            if (gameDir == null) gameDir = AppDomain.CurrentDomain.BaseDirectory;
            if (!Directory.Exists(gameDir)) throw new Exception("game dir not found: " + gameDir);
            if (!string.IsNullOrEmpty(info.GameExe) && !File.Exists(Path.Combine(gameDir, info.GameExe)))
                throw new Exception("game_exe not found in game dir: " + info.GameExe);
            Console.WriteLine("dir   : " + gameDir);
            Console.WriteLine("applying...");
            PatchJob.ApplyAll(info, patchPath, gameDir,
                delegate(long d, long t) { Console.Write("\r" + (d / (1024 * 1024)) + " / " + (t / (1024 * 1024)) + " MB   "); },
                delegate(string s) { Console.WriteLine(s); });
            Console.WriteLine();
            Console.WriteLine("DONE - originals backed up as .ffbak");
            return 0;
        }

        private static int Uninstall(string patchPath, string gameDir)
        {
            PatchInfo info = Load(patchPath);
            if (gameDir == null) gameDir = AppDomain.CurrentDomain.BaseDirectory;
            if (!Directory.Exists(gameDir)) throw new Exception("game dir not found: " + gameDir);
            Console.WriteLine("uninstalling from: " + gameDir);
            PatchJob.UninstallAll(info, gameDir, delegate(string s) { Console.WriteLine(s); });
            Console.WriteLine("DONE - originals restored");
            return 0;
        }
    }
#endif

    public static class Program
    {
        [STAThread]
        public static int Main(string[] args)
        {
#if CLI
            return ConsoleMode.Run(args);
#else
            Application.EnableVisualStyles();
            Application.SetCompatibleTextRenderingDefault(false);
            Application.Run(new MainForm());
            return 0;
#endif
        }
    }
}
