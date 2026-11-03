"use client";

import React, { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth-context";
import { Code2, Lock, Mail, User, ArrowRight, AlertCircle, Coins, CheckCircle2 } from "lucide-react";

export default function RegisterPage() {
  const router = useRouter();
  const { register } = useAuth();
  const [fullName, setFullName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (password.length < 10) {
      setError("Password must be at least 10 characters long.");
      return;
    }

    setIsSubmitting(true);
    try {
      await register(email, password, fullName);
      router.push("/onboarding");
    } catch (err: unknown) {
      const e = err as Error;
      setError(e.message || "Failed to create account. Please try again.");
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="min-h-[85vh] flex items-center justify-center px-4 py-12">
      <div className="w-full max-w-md">
        {/* Card Header */}
        <div className="text-center mb-6">
          <div className="inline-flex h-14 w-14 rounded-2xl bg-[#0e1017]/85 border border-amber-500/30 items-center justify-center shadow-[inset_0_1px_1px_rgba(255,255,255,0.15),0_0_25px_rgba(245,158,11,0.2)] mb-4">
            <Code2 className="h-7 w-7 text-amber-400 stroke-[2.5]" />
          </div>
          <h1 className="text-2xl font-extrabold text-white tracking-tight">Create Your Account</h1>
          <p className="text-xs text-neutral-400 mt-1">
            Start personalized, spaced-repetition DSA interview prep
          </p>
        </div>

        {/* 50 Credits Incentive Glass Banner */}
        <div className="mb-6 p-4 rounded-2xl bg-[#0e1017]/75 backdrop-blur-2xl border border-amber-500/30 shadow-[inset_0_1px_1px_rgba(255,255,255,0.1),0_8px_24px_rgba(245,158,11,0.08)] flex items-center gap-3.5">
          <div className="p-2.5 rounded-xl bg-amber-500/15 text-amber-400 shrink-0 border border-amber-500/30">
            <Coins className="h-5 w-5" />
          </div>
          <div>
            <h2 className="text-xs font-bold text-amber-400 uppercase tracking-wider font-mono">
              Welcome Bonus
            </h2>
            <p className="text-xs text-neutral-300 mt-0.5">
              Get <span className="font-bold text-amber-400 font-mono">50 free AI credits</span>{" "}
              instantly upon registration for progressive hints and code reviews.
            </p>
          </div>
        </div>

        {/* Glass Card Body */}
        <div className="bg-[#0e1017]/75 border border-white/[0.08] rounded-2xl p-6 sm:p-8 backdrop-blur-2xl shadow-[inset_0_1px_1px_rgba(255,255,255,0.1),0_20px_50px_rgba(0,0,0,0.6)]">
          {error && (
            <div className="mb-6 p-4 rounded-xl bg-amber-500/10 border border-amber-500/30 flex items-start gap-3 text-white text-xs">
              <AlertCircle className="h-5 w-5 shrink-0 text-amber-400 mt-0.5" />
              <div>
                <p className="font-semibold text-amber-300">Registration Issue</p>
                <p className="text-neutral-300 mt-0.5">{error}</p>
              </div>
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="block text-xs font-medium text-neutral-300 mb-1.5 uppercase font-mono">
                Full Name
              </label>
              <div className="relative">
                <User className="absolute left-3.5 top-3 h-4 w-4 text-neutral-500" />
                <input
                  type="text"
                  required
                  value={fullName}
                  onChange={(e) => setFullName(e.target.value)}
                  placeholder="Ada Lovelace"
                  className="w-full pl-10 pr-4 py-2.5 rounded-xl bg-[#0a0c12] border border-white/[0.08] focus:border-amber-400/60 text-white placeholder-neutral-500 text-xs focus:outline-none transition-colors"
                />
              </div>
            </div>

            <div>
              <label className="block text-xs font-medium text-neutral-300 mb-1.5 uppercase font-mono">
                Email Address
              </label>
              <div className="relative">
                <Mail className="absolute left-3.5 top-3 h-4 w-4 text-neutral-500" />
                <input
                  type="email"
                  required
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="name@example.com"
                  className="w-full pl-10 pr-4 py-2.5 rounded-xl bg-[#0a0c12] border border-white/[0.08] focus:border-amber-400/60 text-white placeholder-neutral-500 text-xs focus:outline-none transition-colors"
                />
              </div>
            </div>

            <div>
              <label className="block text-xs font-medium text-neutral-300 mb-1.5 uppercase font-mono">
                Password
              </label>
              <div className="relative">
                <Lock className="absolute left-3.5 top-3 h-4 w-4 text-neutral-500" />
                <input
                  type="password"
                  required
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="Minimum 10 characters"
                  className="w-full pl-10 pr-4 py-2.5 rounded-xl bg-[#0a0c12] border border-white/[0.08] focus:border-amber-400/60 text-white placeholder-neutral-500 text-xs focus:outline-none transition-colors"
                />
              </div>
            </div>

            <div className="py-1">
              <div className="flex items-center gap-2 text-[11px] text-neutral-400">
                <CheckCircle2 className="h-3.5 w-3.5 text-amber-400" />
                <span>Includes access to all 539 FAANG canonical problems</span>
              </div>
            </div>

            <button
              type="submit"
              disabled={isSubmitting}
              className="w-full mt-2 flex items-center justify-center gap-2 py-3 px-4 rounded-xl bg-gradient-to-r from-amber-400 to-amber-500 text-black font-bold text-xs transition-all shadow-[0_0_20px_rgba(245,158,11,0.25)] hover:shadow-[0_0_30px_rgba(245,158,11,0.4)] disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {isSubmitting ? "Creating Account..." : "Create Account & Get 50 Credits"}
              <ArrowRight className="h-4 w-4" />
            </button>
          </form>
        </div>

        {/* Footer Link */}
        <p className="text-center text-xs text-neutral-400 mt-6">
          Already have an account?{" "}
          <Link href="/login" className="text-amber-400 hover:text-amber-300 font-semibold underline underline-offset-4">
            Sign in
          </Link>
        </p>
      </div>
    </div>
  );
}
