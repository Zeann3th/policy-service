import type { Metadata } from "next";
import { Public_Sans } from "next/font/google";
import { Shell } from "@/components/Shell";
import "./globals.css";

const sans = Public_Sans({ subsets: ["latin", "vietnamese"], variable: "--font-sans", display: "swap" });

export const metadata: Metadata = {
  title: "Policy Service CMS",
  description: "Configure limits, fees, discounts and tax, and dry-run transactions against live usage.",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={sans.variable}>
      <body>
        <Shell>{children}</Shell>
      </body>
    </html>
  );
}
