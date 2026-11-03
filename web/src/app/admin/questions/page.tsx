"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

export default function AdminQuestionsRedirect() {
  const router = useRouter();

  useEffect(() => {
    router.replace("/admin");
  }, [router]);

  return (
    <div className="p-8 text-neutral-400 text-sm flex items-center justify-center min-h-[50vh]">
      Redirecting to Admin Catalog...
    </div>
  );
}

