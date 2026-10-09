import re

with open(r'D:\FarsiForge\frontend\src\app\page.tsx', 'r', encoding='utf-8') as f:
    content = f.read()

content = content.replace('import { Separator } from "@/components/ui/separator";',
'''import { Separator } from "@/components/ui/separator";
import { SelectDirectory, DetectEngine, Extract, GetProject, SaveTranslations, Inject, BuildPatcher } from "../../../wailsjs/go/main/App";''')

content = content.replace('type Step = "detect" | "extract" | "translate" | "inject" | "success";',
'type Step = "detect" | "extract" | "translate" | "inject" | "patch" | "success";')

state_block = '''  const [currentStep, setCurrentStep] = useState<Step>("detect");
  const [dirPath, setDirPath] = useState("D:\\\\Games\\\\Supermarket Together");
  const [isProcessing, setIsProcessing] = useState(false);
  const [progress, setProgress] = useState(0);

  // Mock Engine Info
  const engineInfo = {
    engine: "Unity Engine",
    backend: "Mono",
    confidence: "95%",
    files: 1420
  };'''

new_state_block = '''  const [currentStep, setCurrentStep] = useState<Step>("detect");
  const [dirPath, setDirPath] = useState("D:\\\\Games\\\\Supermarket Together");
  const [isProcessing, setIsProcessing] = useState(false);
  const [progress, setProgress] = useState(0);
  const [gameInfo, setGameInfo] = useState<any>(null);
  const [translations, setTranslations] = useState<any[]>([]);
  const [patchCredits, setPatchCredits] = useState("");

  const engineInfo = {
    engine: gameInfo ? gameInfo.Engine : "---",
    backend: gameInfo ? gameInfo.Backend : "---",
    confidence: gameInfo ? (gameInfo.Confidence * 100).toFixed(0) + "%" : "---",
    files: "---"
  };

  const handleSelectDir = async () => {
    try {
      const path = await SelectDirectory();
      if (path) setDirPath(path);
    } catch (e) { console.error(e); }
  };
'''
content = content.replace(state_block, new_state_block)

# Handle Next Step
handle_next = '''  const handleNextStep = (next: Step) => {
    setIsProcessing(true);
    setProgress(0);
    
    // Simulate processing
    const interval = setInterval(() => {
      setProgress(p => {
        if (p >= 100) {
          clearInterval(interval);
          setTimeout(() => {
            setIsProcessing(false);
            setCurrentStep(next);
          }, 300);
          return 100;
        }
        return p + 5;
      });
    }, 50);
  };'''

new_handle_next = '''  const handleNextStep = async (next: Step) => {
    setIsProcessing(true);
    setProgress(50);
    try {
      if (next === "extract") {
        const info = await DetectEngine(dirPath);
        setGameInfo(info);
      } else if (next === "translate") {
        await Extract(gameInfo);
        const proj = await GetProject(gameInfo.GameRoot);
        setTranslations(proj.Entries || []);
      } else if (next === "inject") {
        await SaveTranslations(gameInfo.GameRoot, translations);
      } else if (next === "patch") {
        await Inject(gameInfo);
      } else if (next === "success") {
        await BuildPatcher(gameInfo.GameRoot, patchCredits);
      }
      setProgress(100);
      setCurrentStep(next);
    } catch (e) {
      console.error(e);
      alert(String(e));
    } finally {
      setIsProcessing(false);
      setProgress(0);
    }
  };'''
content = content.replace(handle_next, new_handle_next)

# Input Field replace
input_html = '''                      <div className="relative">
                        <FolderOpen className="absolute right-3 top-1/2 -translate-y-1/2 w-5 h-5 text-zinc-500" />
                        <Input 
                          value={dirPath}
                          onChange={(e) => setDirPath(e.target.value)}
                          className="pl-4 pr-10 py-6 bg-zinc-900/50 border-zinc-800 text-left font-mono text-lg focus-visible:ring-indigo-500"
                          dir="ltr"
                        />
                      </div>'''

new_input_html = '''                      <div className="flex gap-2 relative">
                        <div className="relative flex-1">
                          <FolderOpen className="absolute right-3 top-1/2 -translate-y-1/2 w-5 h-5 text-zinc-500" />
                          <Input 
                            value={dirPath}
                            onChange={(e) => setDirPath(e.target.value)}
                            className="pl-4 pr-10 py-6 bg-zinc-900/50 border-zinc-800 text-left font-mono text-lg focus-visible:ring-indigo-500"
                            dir="ltr"
                          />
                        </div>
                        <Button onClick={handleSelectDir} className="h-[52px] px-6 bg-zinc-800 hover:bg-zinc-700">انتخاب مسیر</Button>
                      </div>'''
content = content.replace(input_html, new_input_html)


# Translations mapping
translate_html = '''                    <div className="flex-1 bg-zinc-900/50 rounded-xl border border-zinc-800 p-4 space-y-3 overflow-y-auto max-h-[300px]">
                      {[
                        { id: 's_001', en: 'Play Game', fa: 'شروع بازی', status: 'translated' },
                        { id: 's_002', en: 'Options', fa: 'تنظیمات', status: 'translated' },
                        { id: 's_003', en: 'Quit to Desktop', fa: 'خروج به دسکتاپ', status: 'translated' },
                        { id: 's_004', en: 'Loading...', fa: 'در حال بارگذاری...', status: 'translated' },
                      ].map((item) => (
                        <div key={item.id} className="grid grid-cols-2 gap-4 p-3 rounded-lg bg-zinc-950/50 border border-zinc-800/50">
                          <div className="text-left font-mono text-sm text-zinc-400" dir="ltr">{item.en}</div>
                          <div className="text-right text-sm text-zinc-200">{item.fa}</div>
                        </div>
                      ))}
                    </div>'''

new_translate_html = '''                    <div className="flex-1 bg-zinc-900/50 rounded-xl border border-zinc-800 p-4 space-y-3 overflow-y-auto max-h-[300px]">
                      {translations.length === 0 && <p className="text-center text-zinc-500 py-10">متنی یافت نشد.</p>}
                      {translations.map((item, i) => (
                        <div key={i} className="grid grid-cols-2 gap-4 p-3 rounded-lg bg-zinc-950/50 border border-zinc-800/50">
                          <div className="text-left font-mono text-sm text-zinc-400" dir="ltr">{item.Source}</div>
                          <Input 
                            value={item.Translation || ""} 
                            onChange={(e) => {
                              const newT = [...translations];
                              newT[i].Translation = e.target.value;
                              setTranslations(newT);
                            }}
                            className="bg-zinc-900 border-zinc-700 text-right text-sm text-zinc-200" 
                            placeholder="ترجمه..." 
                          />
                        </div>
                      ))}
                    </div>'''
content = content.replace(translate_html, new_translate_html)

# Inject step modifications
inject_btn_old = '''                      ) : (
                        <Button onClick={() => handleNextStep("success")} size="lg" className="w-full text-lg h-14 bg-emerald-600 hover:bg-emerald-700 shadow-[0_0_20px_rgba(16,185,129,0.3)] transition-all">
                          شروع تزریق به بازی
                        </Button>
                      )}'''
inject_btn_new = '''                      ) : (
                        <Button onClick={() => handleNextStep("patch")} size="lg" className="w-full text-lg h-14 bg-emerald-600 hover:bg-emerald-700 shadow-[0_0_20px_rgba(16,185,129,0.3)] transition-all">
                          شروع تزریق به بازی
                        </Button>
                      )}'''
content = content.replace(inject_btn_old, inject_btn_new)

# Add patch step
patch_html = '''                {/* STEP 5: SUCCESS */}
                {currentStep === "success" && ('''
new_patch_html = '''                {/* STEP 5: PATCH */}
                {currentStep === "patch" && (
                  <motion.div 
                    key="patch"
                    initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}
                    className="flex flex-col items-center text-center space-y-8"
                  >
                    <div className="w-24 h-24 rounded-full bg-orange-500/10 flex items-center justify-center border border-orange-500/20 shadow-[0_0_50px_rgba(249,115,22,0.1)]">
                      <Box className="w-12 h-12 text-orange-400" />
                    </div>
                    <div className="space-y-2">
                      <h2 className="text-2xl font-bold text-white">ساخت نصاب و پچ</h2>
                      <p className="text-zinc-400 max-w-md mx-auto">تزریق با موفقیت انجام شد! برای انتشار، می‌توانید نام مترجمان را وارد کنید تا نصاب نهایی ساخته شود.</p>
                    </div>
                    
                    <div className="w-full max-w-md flex flex-col gap-4">
                      <Input 
                        value={patchCredits} 
                        onChange={(e) => setPatchCredits(e.target.value)} 
                        className="py-6 bg-zinc-900/50 border-zinc-800 text-center text-lg focus-visible:ring-orange-500"
                        placeholder="نام مترجم / تیم (مثلا: تیم ترجمه فلان)"
                      />
                      
                      {isProcessing ? (
                        <div className="space-y-2 w-full">
                          <Progress value={progress} className="h-2 bg-zinc-800" />
                          <p className="text-xs text-zinc-500 text-center animate-pulse">در حال ساخت نصاب...</p>
                        </div>
                      ) : (
                        <Button onClick={() => handleNextStep("success")} size="lg" className="w-full text-lg h-14 bg-orange-600 hover:bg-orange-700 shadow-[0_0_20px_rgba(249,115,22,0.3)] transition-all">
                          ساخت پچ
                        </Button>
                      )}
                    </div>
                  </motion.div>
                )}

                {/* STEP 6: SUCCESS */}
                {currentStep === "success" && ('''
content = content.replace(patch_html, new_patch_html)

with open(r'D:\FarsiForge\frontend\src\app\page.tsx', 'w', encoding='utf-8') as f:
    f.write(content)

print("Patched!")
