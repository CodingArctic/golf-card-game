import type { Metadata } from "next";
import { ThemeProvider } from "@/components/ThemeProvider";
import "./globals.css";

export const metadata: Metadata = {
  title: "Golf Card Game",
  description: "The card game Golf, online!",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className="light">
      <body className="antialiased bg-gray-50 dark:bg-gray-900">
        <ThemeProvider />
        {children}
      </body>
    </html>
  );
}
