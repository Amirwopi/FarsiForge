import type { Metadata } from "next";
import { Vazirmatn } from "next/font/google";
import "./globals.css";

const vazirmatn = Vazirmatn({
  subsets: ["arabic", "latin"],
  variable: "--font-vazirmatn",
  display: "swap",
});

export const metadata: Metadata = {
  title: "FarsiForge | بومی‌ساز پیشرفته بازی‌ها",
  description: "Advanced game localization and translation framework for Persian",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="fa" dir="rtl" className="dark">
      <body
        className={`${vazirmatn.variable} font-sans antialiased bg-zinc-950 text-zinc-50 selection:bg-indigo-500/30`}
      >
        <div className="relative min-h-screen flex flex-col">
          {/* Subtle Cyberpunk Background Glows */}
          <div className="absolute top-[-20%] left-[-10%] w-[50%] h-[50%] rounded-full bg-indigo-900/20 blur-[120px] pointer-events-none" />
          <div className="absolute bottom-[-20%] right-[-10%] w-[50%] h-[50%] rounded-full bg-purple-900/20 blur-[120px] pointer-events-none" />
          
          <main className="flex-1 flex flex-col relative z-10">
            {children}
          </main>
        </div>
      </body>
    </html>
  );
}
