'use client';

import Link from 'next/link';
import { useEffect, useState } from 'react';
import { listGames } from '@/utils/api';

export default function Home() {
  const [isLoggedIn, setIsLoggedIn] = useState(false);

  useEffect(() => {
    // Check if user is logged in by checking for session cookie
    const checkAuth = async () => {
      const response = await listGames();
      setIsLoggedIn(!response.error);
    };
    checkAuth();
  }, []);

  return (
    <div className="font-sans grid grid-rows-[20px_1fr_20px] items-center justify-items-center min-h-screen p-8 pb-20 gap-16 sm:p-20">
      {isLoggedIn && (
        <div className="absolute top-4 right-4">
          <Link
            href="/dash"
            className="rounded-full border border-solid border-blue-600 dark:border-blue-400 transition-colors flex items-center justify-center bg-blue-600 text-white hover:bg-blue-700 dark:bg-blue-500 dark:hover:bg-blue-600 font-medium text-sm h-10 px-5"
          >
            Go to Dashboard →
          </Link>
        </div>
      )}
      <main className="flex flex-col gap-[32px] row-start-2 items-center sm:items-start">
        <h1 className="text-4xl text-center">Welcome to Golf Card Game Online!</h1>
        <div className="flex gap-4 w-full items-center justify-center flex-col  sm:flex-row">
          <a
            className="rounded-full border border-solid border-transparent transition-colors flex items-center justify-center bg-foreground text-background gap-2 hover:bg-[#383838] dark:hover:bg-[#ccc] font-medium text-sm sm:text-base h-10 sm:h-12 px-4 sm:px-5 w-full sm:w-auto md:w-[158px]"
            href="/login"
          >
            Login
          </a>
          <a
            className="rounded-full border border-solid border-black/[.08] dark:border-white/[.145] transition-colors flex items-center justify-center hover:bg-[#f2f2f2] dark:hover:bg-[#1a1a1a] hover:border-transparent font-medium text-sm sm:text-base h-10 sm:h-12 px-4 sm:px-5 w-full sm:w-auto md:w-[158px]"
            href="/register"
          >
            Register
          </a>
        </div>
      </main>
      <footer className="row-start-3 flex gap-[24px] flex-wrap items-center justify-center">

      </footer>
    </div>
  );
}
