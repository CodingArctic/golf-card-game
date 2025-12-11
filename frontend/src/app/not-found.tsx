import Link from "next/link";

export default function NotFound() {
  return (
    <div className="font-sans flex items-center justify-center min-h-screen p-8">
      <div className="text-center">
        <h1 className="text-9xl font-bold text-gray-300 dark:text-gray-700">
          404
        </h1>
        <h2 className="text-4xl font-semibold text-gray-800 dark:text-gray-200 mt-4">
          Page Not Found
        </h2>
        <p className="text-lg text-gray-600 dark:text-gray-400 mt-4 mb-8">
          Oops! The page you're looking for doesn't exist.
        </p>
        <Link
          href="/"
          className="rounded-full border border-solid border-transparent transition-colors flex items-center justify-center bg-foreground text-background gap-2 hover:bg-[#383838] dark:hover:bg-[#ccc] font-medium text-sm sm:text-base h-10 sm:h-12 px-4 sm:px-5 mx-auto w-fit"
        >
          Return Home
        </Link>
      </div>
    </div>
  );
}
