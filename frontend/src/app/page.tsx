"use client";

import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { 
  FolderSearch, 
  Settings2, 
  Terminal, 
  Box, 
  Rocket, 
  CheckCircle2, 
  Cpu, 
  FolderOpen,
  ArrowRight,
  Database,
  Languages,
  Code2
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle, CardFooter } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Progress } from "@/components/ui/progress";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Separator } from "@/components/ui/separator";
import { SelectDirectory, DetectEngine, Extract, GetProject, SaveTranslations, Inject, BuildPatcher } from "../../wailsjs/go/main/App";

type Step = "detect" | "extract" | "translate" | "inject" | "patch" | "success";

export default function FarsiForgeDashboard() {
  const [currentStep, setCurrentStep] = useState<Step>("detect");
  const [dirPath, setDirPath] = useState("D:\\Games\\Supermarket Together");
  const [isProcessing, setIsProcessing] = useState(false);
  const [progress, setProgress] = useState(0);
  const [gameInfo, setGameInfo] = useState<any>(null);
  const [translations, setTranslations] = useState<any[]>([]);
  const [patchCredits, setPatchCredits] = useState("");

  const engineInfo = {
    engine: gameInfo ? gameInfo.engine : "---",
    backend: gameInfo ? gameInfo.backend : "---",
    confidence: gameInfo ? (gameInfo.confidence * 100).toFixed(0) + "%" : "---",
    files: "---"
  };

  const handleSelectDir = async () => {
    try {
      const path = await SelectDirectory();
      if (path) setDirPath(path);
    } catch (e) { console.error(e); }
  };


  const handleNextStep = async (next: Step) => {
    setIsProcessing(true);
    setProgress(50);
    try {
      if (next === "extract") {
        const info = await DetectEngine(dirPath);
        setGameInfo(info);
      } else if (next === "translate") {
        await Extract(gameInfo);
        const proj = await GetProject(gameInfo.game_root);
        setTranslations(proj.entries || []);
      } else if (next === "inject") {
        await SaveTranslations(gameInfo.game_root, translations);
      } else if (next === "patch") {
        await Inject(gameInfo);
      } else if (next === "success") {
        await BuildPatcher(gameInfo.game_root, patchCredits);
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
  };

  return (
    <div className="min-h-screen p-6 flex items-center justify-center relative overflow-hidden">
      <div className="absolute inset-0 bg-[url('https://transparenttextures.com/patterns/cubes.png')] opacity-10 pointer-events-none mix-blend-overlay"></div>
      
      <div className="max-w-5xl w-full grid grid-cols-1 lg:grid-cols-12 gap-8 z-10">
        
        {/* Sidebar / Info Panel */}
        <div className="lg:col-span-4 flex flex-col gap-6">
          <div className="flex items-center gap-4 mb-4">
            <div className="p-3 bg-primary/10 rounded-2xl border border-primary/20 backdrop-blur-md">
              <Cpu className="w-8 h-8 text-primary" />
            </div>
            <div>
              <h1 className="text-3xl font-black tracking-tight text-white">FarsiForge</h1>
              <p className="text-zinc-400 font-medium text-sm tracking-widest uppercase">Persian Game Engine</p>
            </div>
          </div>

          <Card className="bg-zinc-950/40 backdrop-blur-xl border-zinc-800/50 shadow-2xl">
            <CardHeader>
              <CardTitle className="text-lg flex items-center gap-2">
                <Box className="w-5 h-5 text-indigo-400" /> 
                مشخصات بازی
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex justify-between items-center py-2 border-b border-zinc-800/50">
                <span className="text-zinc-400">موتور بازی</span>
                <Badge variant="outline" className="bg-indigo-500/10 text-indigo-300 border-indigo-500/20">{currentStep !== "detect" ? engineInfo.engine : "---"}</Badge>
              </div>
              <div className="flex justify-between items-center py-2 border-b border-zinc-800/50">
                <span className="text-zinc-400">پلتفرم پردازشی</span>
                <span className="font-mono text-zinc-300">{currentStep !== "detect" ? engineInfo.backend : "---"}</span>
              </div>
              <div className="flex justify-between items-center py-2 border-b border-zinc-800/50">
                <span className="text-zinc-400">دقت شناسایی</span>
                <span className="text-green-400 font-bold">{currentStep !== "detect" ? engineInfo.confidence : "---"}</span>
              </div>
              <div className="flex justify-between items-center py-2">
                <span className="text-zinc-400">تعداد فایل‌ها</span>
                <span className="font-mono text-zinc-300">{currentStep !== "detect" ? engineInfo.files : "---"}</span>
              </div>
            </CardContent>
          </Card>

          <Card className="bg-zinc-950/40 backdrop-blur-xl border-zinc-800/50">
            <CardHeader>
              <CardTitle className="text-lg flex items-center gap-2">
                <Settings2 className="w-5 h-5 text-purple-400" />
                تنظیمات بومی‌سازی
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              {['راست‌چین‌سازی (RTL)', 'اتصال حروف فارسی', 'اعداد فارسی', 'فونت Vazirmatn'].map((setting) => (
                <div key={setting} className="flex items-center gap-3">
                  <div className="w-4 h-4 rounded-sm bg-primary border-primary flex items-center justify-center">
                    <CheckCircle2 className="w-3 h-3 text-primary-foreground" />
                  </div>
                  <span className="text-sm text-zinc-300">{setting}</span>
                </div>
              ))}
            </CardContent>
          </Card>
        </div>

        {/* Main Work Area */}
        <div className="lg:col-span-8 flex flex-col">
          <Card className="flex-1 bg-zinc-950/60 backdrop-blur-2xl border-zinc-800/60 shadow-2xl relative overflow-hidden flex flex-col">
            {/* Dynamic header gradient */}
            <div className="absolute top-0 inset-x-0 h-1 bg-gradient-to-r from-transparent via-primary to-transparent opacity-50" />
            
            <CardHeader className="border-b border-zinc-800/50 pb-6 pt-8">
              <div className="flex justify-between items-center">
                <CardTitle className="text-2xl font-bold">پایپ‌لاین بومی‌سازی</CardTitle>
                <div className="flex gap-2">
                  <Badge variant={currentStep === "detect" ? "default" : "outline"}>شناسایی</Badge>
                  <ArrowRight className="w-4 h-4 text-zinc-600 mt-0.5" />
                  <Badge variant={currentStep === "extract" ? "default" : "outline"}>استخراج</Badge>
                  <ArrowRight className="w-4 h-4 text-zinc-600 mt-0.5" />
                  <Badge variant={currentStep === "inject" ? "default" : "outline"}>تزریق</Badge>
                </div>
              </div>
            </CardHeader>

            <CardContent className="flex-1 flex flex-col justify-center p-8 relative">
              <AnimatePresence mode="wait">
                
                {/* STEP 1: DETECT */}
                {currentStep === "detect" && (
                  <motion.div 
                    key="detect"
                    initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}
                    className="flex flex-col items-center text-center space-y-8"
                  >
                    <div className="w-24 h-24 rounded-full bg-indigo-500/10 flex items-center justify-center border border-indigo-500/20 shadow-[0_0_50px_rgba(99,102,241,0.1)]">
                      <FolderSearch className="w-12 h-12 text-indigo-400" />
                    </div>
                    <div className="space-y-2">
                      <h2 className="text-2xl font-bold text-white">انتخاب مسیر بازی</h2>
                      <p className="text-zinc-400 max-w-md mx-auto">برای شروع عملیات، مسیر فولدر بازی را انتخاب کنید تا موتور بازی و فایل‌های قابل ترجمه شناسایی شوند.</p>
                    </div>
                    
                    <div className="w-full max-w-md flex flex-col gap-4">
                      <div className="flex gap-2 relative">
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
                      </div>
                      
                      {isProcessing ? (
                        <div className="space-y-2 w-full">
                          <Progress value={progress} className="h-2" />
                          <p className="text-xs text-zinc-500 text-center animate-pulse">در حال تحلیل ساختار فایل‌ها...</p>
                        </div>
                      ) : (
                        <Button onClick={() => handleNextStep("extract")} size="lg" className="w-full text-lg h-14 bg-indigo-600 hover:bg-indigo-700 shadow-[0_0_20px_rgba(79,70,229,0.3)] transition-all">
                          شناسایی موتور
                        </Button>
                      )}
                    </div>
                  </motion.div>
                )}

                {/* STEP 2: EXTRACT */}
                {currentStep === "extract" && (
                  <motion.div 
                    key="extract"
                    initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}
                    className="flex flex-col items-center text-center space-y-8"
                  >
                    <div className="w-24 h-24 rounded-full bg-purple-500/10 flex items-center justify-center border border-purple-500/20 shadow-[0_0_50px_rgba(168,85,247,0.1)]">
                      <Database className="w-12 h-12 text-purple-400" />
                    </div>
                    <div className="space-y-2">
                      <h2 className="text-2xl font-bold text-white">استخراج دیتابیس متن‌ها</h2>
                      <p className="text-zinc-400 max-w-md mx-auto">موتور {engineInfo.engine} با موفقیت شناسایی شد. اکنون فایل‌های متنی و دارایی‌ها (Assets) را استخراج می‌کنیم.</p>
                    </div>
                    
                    <div className="w-full max-w-md flex flex-col gap-4">
                      {isProcessing ? (
                        <div className="space-y-2 w-full">
                          <Progress value={progress} className="h-2 bg-zinc-800" />
                          <p className="text-xs text-zinc-500 text-center animate-pulse">در حال استخراج (resources.assets) ...</p>
                        </div>
                      ) : (
                        <Button onClick={() => handleNextStep("translate")} size="lg" className="w-full text-lg h-14 bg-purple-600 hover:bg-purple-700 shadow-[0_0_20px_rgba(168,85,247,0.3)] transition-all">
                          شروع استخراج
                        </Button>
                      )}
                    </div>
                  </motion.div>
                )}

                {/* STEP 3: TRANSLATE MOCK */}
                {currentStep === "translate" && (
                  <motion.div 
                    key="translate"
                    initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}
                    className="w-full h-full flex flex-col space-y-6"
                  >
                    <div className="flex items-center gap-3">
                      <div className="p-2 bg-blue-500/10 rounded-lg border border-blue-500/20">
                        <Languages className="w-6 h-6 text-blue-400" />
                      </div>
                      <h2 className="text-xl font-bold text-white">مدیریت ترجمه‌ها</h2>
                    </div>

                    <div className="flex-1 bg-zinc-900/50 rounded-xl border border-zinc-800 p-4 space-y-3 overflow-y-auto max-h-[300px]">
                      {translations.length === 0 && <p className="text-center text-zinc-500 py-10">متنی یافت نشد.</p>}
                      {translations.map((item, i) => (
                        <div key={i} className="grid grid-cols-2 gap-4 p-3 rounded-lg bg-zinc-950/50 border border-zinc-800/50">
                          <div className="text-left font-mono text-sm text-zinc-400" dir="ltr">{item.source}</div>
                          <Input 
                            value={item.translation || ""} 
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
                    </div>
                    
                    <div className="flex justify-end pt-2">
                      <Button onClick={() => handleNextStep("inject")} size="lg" className="bg-blue-600 hover:bg-blue-700">
                        آماده‌سازی برای تزریق
                      </Button>
                    </div>
                  </motion.div>
                )}

                {/* STEP 4: INJECT */}
                {currentStep === "inject" && (
                  <motion.div 
                    key="inject"
                    initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}
                    className="flex flex-col items-center text-center space-y-8"
                  >
                    <div className="w-24 h-24 rounded-full bg-emerald-500/10 flex items-center justify-center border border-emerald-500/20 shadow-[0_0_50px_rgba(16,185,129,0.1)] relative">
                      <Code2 className="w-12 h-12 text-emerald-400 relative z-10" />
                      {isProcessing && <div className="absolute inset-0 rounded-full border-t-2 border-emerald-500 animate-spin" />}
                    </div>
                    <div className="space-y-2">
                      <h2 className="text-2xl font-bold text-white">تزریق و اعمال تغییرات</h2>
                      <p className="text-zinc-400 max-w-md mx-auto">در این مرحله تمام متون فارسی شده، راست‌چین شده و فونت‌ها به درون فایل‌های اصلی بازی بازگردانده می‌شوند.</p>
                    </div>
                    
                    <div className="w-full max-w-md flex flex-col gap-4">
                      {isProcessing ? (
                        <div className="space-y-2 w-full">
                          <Progress value={progress} className="h-2 bg-zinc-800" />
                          <p className="text-xs text-zinc-500 text-center animate-pulse">در حال تزریق (RTL Shaping) ...</p>
                        </div>
                      ) : (
                        <Button onClick={() => handleNextStep("patch")} size="lg" className="w-full text-lg h-14 bg-emerald-600 hover:bg-emerald-700 shadow-[0_0_20px_rgba(16,185,129,0.3)] transition-all">
                          شروع تزریق به بازی
                        </Button>
                      )}
                    </div>
                  </motion.div>
                )}

                {/* STEP 5: PATCH */}
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
                {currentStep === "success" && (
                  <motion.div 
                    key="success"
                    initial={{ opacity: 0, scale: 0.95 }} animate={{ opacity: 1, scale: 1 }}
                    className="flex flex-col items-center text-center space-y-8"
                  >
                    <div className="w-24 h-24 rounded-full bg-green-500/20 flex items-center justify-center border border-green-500/30 shadow-[0_0_80px_rgba(34,197,94,0.2)]">
                      <Rocket className="w-12 h-12 text-green-400" />
                    </div>
                    <div className="space-y-2">
                      <h2 className="text-3xl font-black text-white">عملیات با موفقیت انجام شد!</h2>
                      <p className="text-zinc-400 max-w-md mx-auto text-lg">فارسی‌ساز با موفقیت بر روی بازی اعمال شد. اکنون می‌توانید بازی را اجرا کنید.</p>
                    </div>
                    
                    <div className="flex gap-4 mt-4">
                      <Button onClick={() => setCurrentStep("detect")} variant="outline" className="border-zinc-700 hover:bg-zinc-800">
                        شروع پروژه‌ی جدید
                      </Button>
                      <Button className="bg-green-600 hover:bg-green-700 px-8">
                        اجرای بازی
                      </Button>
                    </div>
                  </motion.div>
                )}

              </AnimatePresence>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
