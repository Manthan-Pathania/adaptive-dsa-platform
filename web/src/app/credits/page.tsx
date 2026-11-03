"use client";

import React, { useEffect, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth-context";
import { api } from "@/lib/api";
import { CreditPackage, LedgerEntry, PaymentOrder } from "@/types";
import {
  Coins,
  Sparkles,
  ArrowDownLeft,
  ArrowUpRight,
  ShieldCheck,
  RefreshCw,
  Clock,
  CheckCircle2,
  AlertTriangle,
  CreditCard,
  Zap,
  Check,
  Loader2,
  History,
} from "lucide-react";

declare global {
  interface Window {
    Razorpay?: new (options: Record<string, unknown>) => {
      open: () => void;
      on: (event: string, handler: (resp: unknown) => void) => void;
    };
  }
}

export default function CreditsPage() {
  const router = useRouter();
  const { user, credits, refreshCredits, loading: authLoading } = useAuth();

  // Data state
  const [packages, setPackages] = useState<CreditPackage[]>([]);
  const [entries, setEntries] = useState<LedgerEntry[]>([]);
  const [paymentOrders, setPaymentOrders] = useState<PaymentOrder[]>([]);
  const [ledgerTotal, setLedgerTotal] = useState(0);
  const [historyTotal, setHistoryTotal] = useState(0);

  // UI state
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<"packages" | "history">("packages");

  // Purchase state
  const [selectedPackageId, setSelectedPackageId] = useState<string>("pkg_pro");
  const [purchaseStatus, setPurchaseStatus] = useState<
    "idle" | "creating_order" | "modal_open" | "verifying" | "success" | "failed"
  >("idle");
  const [purchaseError, setPurchaseError] = useState<string | null>(null);
  const [purchasedCreditsCount, setPurchasedCreditsCount] = useState<number | null>(null);

  // Load Razorpay Checkout SDK Script
  const loadRazorpayScript = (): Promise<boolean> => {
    return new Promise((resolve) => {
      if (typeof window !== "undefined" && window.Razorpay) {
        resolve(true);
        return;
      }
      const existingScript = document.getElementById("razorpay-checkout-sdk");
      if (existingScript) {
        resolve(true);
        return;
      }
      const script = document.createElement("script");
      script.id = "razorpay-checkout-sdk";
      script.src = "https://checkout.razorpay.com/v1/checkout.js";
      script.async = true;
      script.onload = () => resolve(true);
      script.onerror = () => resolve(false);
      document.body.appendChild(script);
    });
  };

  const fetchData = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const [pkgsRes, ledgerRes, historyRes] = await Promise.all([
        api.getPaymentPackages(),
        api.getCreditLedger(15, 0),
        api.getPaymentHistory(15, 0),
      ]);
      setPackages(pkgsRes.packages);
      setEntries(ledgerRes.entries);
      setLedgerTotal(ledgerRes.total);
      setPaymentOrders(historyRes.orders);
      setHistoryTotal(historyRes.total);
    } catch (err: unknown) {
      const e = err as Error;
      setError(e.message || "Failed to load credits information.");
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!authLoading && !user) {
      router.push("/login");
      return;
    }
    if (user) {
      fetchData();
    }
  }, [user, authLoading, router, fetchData]);

  // Handle Checkout / Razorpay Integration
  const handlePurchase = async (pkg: CreditPackage) => {
    if (!user) {
      router.push("/login");
      return;
    }

    setSelectedPackageId(pkg.id);
    setPurchaseStatus("creating_order");
    setPurchaseError(null);

    const sdkLoaded = await loadRazorpayScript();
    if (!sdkLoaded) {
      setPurchaseStatus("failed");
      setPurchaseError("Unable to load Razorpay payment gateway. Please check your connection.");
      return;
    }

    try {
      const order = await api.createPaymentOrder(pkg.id);

      setPurchaseStatus("modal_open");

      if (!window.Razorpay) {
        throw new Error("Razorpay SDK not initialized.");
      }

      const options = {
        key: order.key_id,
        amount: order.amount_paise,
        currency: order.currency,
        name: "AdaptiveDSA Platform",
        description: `${pkg.name} - ${pkg.credits} AI Tutoring Credits`,
        order_id: order.razorpay_order_id,
        handler: async function (response: {
          razorpay_payment_id: string;
          razorpay_order_id: string;
          razorpay_signature: string;
        }) {
          setPurchaseStatus("verifying");
          try {
            await api.verifyPayment({
              razorpay_payment_id: response.razorpay_payment_id,
              razorpay_order_id: response.razorpay_order_id,
              razorpay_signature: response.razorpay_signature,
            });

            await refreshCredits();
            setPurchasedCreditsCount(pkg.credits);
            setPurchaseStatus("success");
            fetchData();
          } catch (err: unknown) {
            const e = err as Error;
            setPurchaseStatus("failed");
            setPurchaseError(e.message || "Payment verification failed.");
          }
        },
        prefill: {
          email: user.email,
          name: user.full_name,
        },
        theme: {
          color: "#F59E0B",
        },
        modal: {
          ondismiss: function () {
            setPurchaseStatus((prev) => (prev === "modal_open" ? "idle" : prev));
          },
        },
      };

      const rzp = new window.Razorpay(options);
      rzp.open();
    } catch (err: unknown) {
      const e = err as Error;
      setPurchaseStatus("failed");
      setPurchaseError(e.message || "Failed to initialize payment order.");
    }
  };

  const getEntryBadge = (type: string) => {
    switch (type) {
      case "PROMOTIONAL_GRANT":
        return {
          label: "Promotional Grant",
          className: "bg-amber-500/15 text-amber-300 border-amber-500/30",
          icon: Sparkles,
        };
      case "PURCHASE":
        return {
          label: "Purchase",
          className: "bg-amber-500/15 text-amber-300 border-amber-500/30",
          icon: Coins,
        };
      case "AI_USAGE":
        return {
          label: "AI Tutoring Usage",
          className: "bg-[#181a22] text-neutral-300 border-white/[0.08]",
          icon: ArrowUpRight,
        };
      case "REFUND":
        return {
          label: "Refund",
          className: "bg-[#181a22] text-neutral-300 border-white/[0.08]",
          icon: ArrowDownLeft,
        };
      case "ADMIN_ADJUSTMENT":
      default:
        return {
          label: "Admin Adjustment",
          className: "bg-amber-500/15 text-amber-300 border-amber-500/30",
          icon: ShieldCheck,
        };
    }
  };

  const getOrderStatusBadge = (status: string) => {
    switch (status) {
      case "PAID":
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-[10px] font-semibold bg-amber-500/15 text-amber-300 border border-amber-500/30">
            <CheckCircle2 className="h-3 w-3" />
            PAID
          </span>
        );
      case "ATTEMPTED":
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-[10px] font-semibold bg-[#181a22] text-neutral-400 border border-white/[0.08]">
            <Clock className="h-3 w-3" />
            ATTEMPTED
          </span>
        );
      case "FAILED":
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-[10px] font-semibold bg-[#181a22] text-neutral-400 border border-white/[0.08]">
            <AlertTriangle className="h-3 w-3" />
            FAILED
          </span>
        );
      case "CREATED":
      default:
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-[10px] font-semibold bg-[#181a22] text-neutral-500 border border-white/[0.08]">
            CREATED
          </span>
        );
    }
  };

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 w-full space-y-8 text-white min-h-screen">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-2 text-xs font-mono text-amber-400 mb-1">
            <Coins className="h-4 w-4" />
            <span>TRANSPARENT LEDGER &amp; RAZORPAY TEST MODE</span>
          </div>
          <h1 className="text-2xl sm:text-3xl font-extrabold text-white tracking-tight">
            Credits &amp; AI Billing
          </h1>
          <p className="text-sm text-neutral-400 mt-0.5">
            Purchase AI tutoring credits securely with immutable double-entry ledger accounting.
          </p>
        </div>

        <button
          onClick={fetchData}
          disabled={isLoading}
          className="inline-flex items-center gap-2 px-4 py-2 rounded-xl bg-[#0e1017]/75 backdrop-blur-2xl border border-white/[0.08] hover:border-amber-400/40 text-xs font-medium text-neutral-300 hover:text-white transition-colors self-start sm:self-auto cursor-pointer shadow-[inset_0_1px_1px_rgba(255,255,255,0.08)]"
        >
          <RefreshCw className={`h-3.5 w-3.5 ${isLoading ? "animate-spin text-amber-400" : ""}`} />
          Refresh
        </button>
      </div>

      {error && (
        <div className="p-4 rounded-xl bg-amber-500/10 border border-amber-500/30 text-amber-200 text-sm flex items-center gap-2 backdrop-blur-md">
          <AlertTriangle className="h-4 w-4 text-amber-400" />
          <span>{error}</span>
        </div>
      )}

      {/* Balance Summary Glass Cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        <div className="p-6 rounded-2xl bg-[#0e1017]/75 backdrop-blur-2xl border border-amber-500/30 shadow-[inset_0_1px_1px_rgba(255,255,255,0.12),0_12px_32px_rgba(245,158,11,0.08)] relative overflow-hidden">
          <div className="absolute top-0 right-0 w-40 h-40 bg-amber-500/10 rounded-full blur-2xl pointer-events-none" />
          <div className="flex items-center justify-between text-neutral-400 mb-2 relative z-10">
            <span className="text-xs font-mono uppercase tracking-wider text-amber-400 font-semibold">
              Current Available Balance
            </span>
            <Sparkles className="h-4 w-4 text-amber-400" />
          </div>
          <div className="flex items-baseline gap-2 relative z-10">
            <span className="text-4xl font-extrabold text-white font-mono">{credits}</span>
            <span className="text-xs text-neutral-400">credits available</span>
          </div>
          <p className="text-[11px] text-neutral-400 mt-3 relative z-10">
            Deducted only upon confirmed AI completion (Hints: 2–4 cr, Code Review: 5 cr, Concept: 3 cr).
          </p>
        </div>

        <div className="p-6 rounded-2xl bg-[#0e1017]/75 backdrop-blur-2xl border border-white/[0.08] shadow-[inset_0_1px_1px_rgba(255,255,255,0.08)] flex flex-col justify-between">
          <div className="flex items-center justify-between text-neutral-400 mb-2">
            <span className="text-xs font-mono uppercase tracking-wider">Zero-Charge Guarantee</span>
            <ShieldCheck className="h-4 w-4 text-amber-400" />
          </div>
          <p className="text-xs text-neutral-300 leading-relaxed">
            Credits are never deducted on API failures, quota errors, or network timeouts. Every grant and deduction is verified cryptographically.
          </p>
          <div className="flex items-center gap-1.5 text-[11px] text-amber-400 font-mono mt-3">
            <CheckCircle2 className="h-3.5 w-3.5" />
            <span>Server-Side Pricing Enforced</span>
          </div>
        </div>

        <div className="p-6 rounded-2xl bg-[#0e1017]/75 backdrop-blur-2xl border border-white/[0.08] shadow-[inset_0_1px_1px_rgba(255,255,255,0.08)] flex flex-col justify-between">
          <div className="flex items-center justify-between text-neutral-400 mb-2">
            <span className="text-xs font-mono uppercase tracking-wider">Razorpay Test Gateway</span>
            <CreditCard className="h-4 w-4 text-amber-400" />
          </div>
          <p className="text-xs text-neutral-300 leading-relaxed">
            Test mode enabled. Use simulated cards or mock test payments. All monetary amounts stored as integer paise.
          </p>
          <div className="text-[11px] text-amber-400 font-mono mt-3">
            HMAC-SHA256 Webhook Protected
          </div>
        </div>
      </div>

      {/* Purchase Feedback Alerts */}
      {purchaseStatus === "creating_order" && (
        <div className="p-4 rounded-xl bg-[#0e1017]/85 backdrop-blur-2xl border border-amber-500/30 text-neutral-200 flex items-center gap-3 text-sm">
          <Loader2 className="h-5 w-5 animate-spin flex-shrink-0 text-amber-400" />
          <span>Contacting server to generate cryptographic payment order...</span>
        </div>
      )}

      {purchaseStatus === "modal_open" && (
        <div className="p-4 rounded-xl bg-[#0e1017]/85 backdrop-blur-2xl border border-amber-500/40 text-neutral-200 flex items-center gap-3 text-sm">
          <Clock className="h-5 w-5 animate-pulse flex-shrink-0 text-amber-400" />
          <span>Razorpay Checkout modal is active. Please complete payment in test mode.</span>
        </div>
      )}

      {purchaseStatus === "verifying" && (
        <div className="p-4 rounded-xl bg-[#0e1017]/85 backdrop-blur-2xl border border-amber-500/40 text-neutral-200 flex items-center gap-3 text-sm">
          <Loader2 className="h-5 w-5 animate-spin flex-shrink-0 text-amber-400" />
          <span>Verifying payment signature with banking gateway and granting credits...</span>
        </div>
      )}

      {purchaseStatus === "success" && (
        <div className="p-4 rounded-xl bg-[#0e1017]/90 backdrop-blur-2xl border border-amber-500/40 text-neutral-200 flex items-center justify-between gap-3 text-sm">
          <div className="flex items-center gap-3">
            <CheckCircle2 className="h-5 w-5 flex-shrink-0 text-amber-400" />
            <div>
              <p className="font-semibold text-white">Payment Verified Successfully!</p>
              <p className="text-xs text-neutral-400">
                Granted {purchasedCreditsCount} AI credits to your account. Your balance has been updated.
              </p>
            </div>
          </div>
          <button
            onClick={() => setPurchaseStatus("idle")}
            className="text-xs px-2.5 py-1 rounded-lg bg-amber-500/20 hover:bg-amber-500/30 text-amber-300 transition-colors cursor-pointer"
          >
            Dismiss
          </button>
        </div>
      )}

      {purchaseStatus === "failed" && purchaseError && (
        <div className="p-4 rounded-xl bg-[#0e1017]/90 backdrop-blur-2xl border border-red-500/30 text-neutral-300 flex items-center justify-between gap-3 text-sm">
          <div className="flex items-center gap-3">
            <AlertTriangle className="h-5 w-5 flex-shrink-0 text-amber-400" />
            <div>
              <p className="font-semibold text-white">Checkout Error</p>
              <p className="text-xs text-neutral-400">{purchaseError}</p>
            </div>
          </div>
          <button
            onClick={() => setPurchaseStatus("idle")}
            className="text-xs px-3 py-1 rounded-lg bg-gradient-to-r from-amber-500 to-amber-600 text-black font-semibold transition-colors cursor-pointer shadow-sm"
          >
            Retry
          </button>
        </div>
      )}

      {/* Credit Packages Section */}
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-lg font-bold text-white flex items-center gap-2">
              <Zap className="h-5 w-5 text-amber-400" />
              Purchase AI Credits
            </h2>
            <p className="text-xs text-neutral-400">
              Select a tier below. Pricing is defined and enforced by the server; amounts are stored in INR paise.
            </p>
          </div>
          <span className="text-xs font-mono px-2.5 py-1 rounded-full bg-[#181a22] text-neutral-300 border border-white/[0.08]">
            TEST MODE
          </span>
        </div>

        {packages.length === 0 && isLoading ? (
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6 animate-pulse">
            {[1, 2, 3].map((i) => (
              <div key={i} className="h-64 bg-[#0e1017]/60 rounded-2xl border border-white/[0.06]" />
            ))}
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            {packages.map((pkg) => {
              const isSelected = selectedPackageId === pkg.id;
              const priceINR = (pkg.amount_paise / 100).toFixed(0);
              const perCredit = (pkg.amount_paise / (pkg.credits * 100)).toFixed(2);

              return (
                <div
                  key={pkg.id}
                  onClick={() => setSelectedPackageId(pkg.id)}
                  className={`relative p-6 rounded-2xl flex flex-col justify-between transition-all cursor-pointer backdrop-blur-2xl shadow-[inset_0_1px_1px_rgba(255,255,255,0.08)] ${
                    pkg.popular
                      ? "bg-[#10131d]/90 border-2 border-amber-400/80 shadow-[inset_0_1px_1px_rgba(255,255,255,0.15),0_12px_32px_rgba(245,158,11,0.15)]"
                      : isSelected
                      ? "bg-[#0e1017]/90 border border-amber-400/60 shadow-[0_0_20px_rgba(245,158,11,0.1)]"
                      : "bg-[#0e1017]/75 border border-white/[0.08] hover:border-amber-400/30 hover:bg-[#121520]/80"
                  }`}
                >
                  {pkg.popular && (
                    <div className="absolute -top-3 left-1/2 -translate-x-1/2 px-3 py-0.5 rounded-full bg-gradient-to-r from-amber-400 to-amber-500 text-black font-extrabold text-[10px] uppercase tracking-wider shadow-md">
                      Most Popular
                    </div>
                  )}

                  <div className="space-y-4">
                    <div className="flex items-center justify-between">
                      <h3 className="text-base font-bold text-white">{pkg.name}</h3>
                      <span className="text-xs font-mono text-neutral-400">₹{perCredit}/cr</span>
                    </div>

                    <div className="flex items-baseline gap-1">
                      <span className="text-3xl font-extrabold text-white font-mono">₹{priceINR}</span>
                      <span className="text-xs text-neutral-400">INR</span>
                    </div>

                    <div className="p-3 rounded-xl bg-[#0a0c12] border border-amber-500/25 flex items-center justify-between">
                      <span className="text-xs text-neutral-300 font-medium">Credits Included</span>
                      <span className="text-lg font-bold font-mono text-amber-400">
                        {pkg.credits} cr
                      </span>
                    </div>

                    <p className="text-xs text-neutral-400 leading-relaxed min-h-[36px]">
                      {pkg.description}
                    </p>

                    <div className="space-y-2 pt-2 border-t border-white/[0.06] text-xs text-neutral-300">
                      <div className="flex items-center gap-2">
                        <Check className="h-3.5 w-3.5 text-amber-400 flex-shrink-0" />
                        <span>Up to {Math.floor(pkg.credits / 2)} Progressive Hints</span>
                      </div>
                      <div className="flex items-center gap-2">
                        <Check className="h-3.5 w-3.5 text-amber-400 flex-shrink-0" />
                        <span>Up to {Math.floor(pkg.credits / 5)} Deep Code Reviews</span>
                      </div>
                      <div className="flex items-center gap-2">
                        <Check className="h-3.5 w-3.5 text-amber-400 flex-shrink-0" />
                        <span>Lifetime validity &amp; zero-charge protection</span>
                      </div>
                    </div>
                  </div>

                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      handlePurchase(pkg);
                    }}
                    disabled={purchaseStatus === "creating_order" || purchaseStatus === "verifying"}
                    className={`mt-6 w-full py-2.5 px-4 rounded-xl text-xs font-bold transition-all flex items-center justify-center gap-2 cursor-pointer ${
                      pkg.popular
                        ? "bg-gradient-to-r from-amber-400 to-amber-500 text-black shadow-[0_0_20px_rgba(245,158,11,0.25)] hover:shadow-[0_0_30px_rgba(245,158,11,0.4)]"
                        : "bg-[#141722]/80 hover:bg-[#1a1e2c] text-white border border-white/[0.1] hover:border-amber-400/40"
                    }`}
                  >
                    {purchaseStatus === "creating_order" && selectedPackageId === pkg.id ? (
                      <>
                        <Loader2 className="h-4 w-4 animate-spin text-black" />
                        <span>Generating Order...</span>
                      </>
                    ) : (
                      <>
                        <CreditCard className="h-4 w-4" />
                        <span>Buy {pkg.name}</span>
                      </>
                    )}
                  </button>
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* Tabs: Transaction Ledger & Payment Order History */}
      <div className="space-y-4">
        <div className="flex items-center gap-2 border-b border-white/[0.08] pb-2">
          <button
            onClick={() => setActiveTab("packages")}
            className={`flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-semibold transition-colors cursor-pointer ${
              activeTab === "packages"
                ? "bg-amber-500/15 text-amber-300 border border-amber-500/40 shadow-sm"
                : "text-neutral-400 hover:text-white border border-transparent"
            }`}
          >
            <Clock className="h-3.5 w-3.5 text-amber-400" />
            Credit Ledger ({ledgerTotal})
          </button>
          <button
            onClick={() => setActiveTab("history")}
            className={`flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-semibold transition-colors cursor-pointer ${
              activeTab === "history"
                ? "bg-amber-500/15 text-amber-300 border border-amber-500/40 shadow-sm"
                : "text-neutral-400 hover:text-white border border-transparent"
            }`}
          >
            <History className="h-3.5 w-3.5 text-amber-400" />
            Payment History ({historyTotal})
          </button>
        </div>

        {activeTab === "packages" ? (
          /* Ledger Table */
          <div className="p-6 rounded-2xl bg-[#0e1017]/75 backdrop-blur-2xl border border-white/[0.08] space-y-4 shadow-[inset_0_1px_1px_rgba(255,255,255,0.08),0_12px_32px_rgba(0,0,0,0.35)]">
            <h2 className="text-base font-bold text-white flex items-center gap-2">
              <Clock className="h-4 w-4 text-amber-400" />
              Immutable Transaction Ledger
            </h2>

            {isLoading ? (
              <div className="space-y-3 animate-pulse">
                {[1, 2, 3, 4].map((i) => (
                  <div key={i} className="h-14 bg-white/[0.04] rounded-xl border border-white/[0.06]" />
                ))}
              </div>
            ) : entries.length > 0 ? (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs border-collapse">
                  <thead>
                    <tr className="border-b border-white/[0.08] text-neutral-400 font-mono uppercase text-[11px]">
                      <th className="py-3 px-4">Date / Time</th>
                      <th className="py-3 px-4">Type</th>
                      <th className="py-3 px-4">Description</th>
                      <th className="py-3 px-4 text-right">Amount</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-white/[0.04]">
                    {entries.map((entry) => {
                      const badge = getEntryBadge(entry.entry_type);
                      const BadgeIcon = badge.icon;
                      const isPositive = entry.amount > 0;
                      return (
                        <tr key={entry.id} className="hover:bg-white/[0.03] transition-colors">
                          <td className="py-3.5 px-4 font-mono text-neutral-400 whitespace-nowrap">
                            {new Date(entry.created_at).toLocaleString()}
                          </td>
                          <td className="py-3.5 px-4 whitespace-nowrap">
                            <span
                              className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-semibold border ${badge.className}`}
                            >
                              <BadgeIcon className="h-3 w-3" />
                              {badge.label}
                            </span>
                          </td>
                          <td className="py-3.5 px-4 text-neutral-300">
                            <span>{entry.description}</span>
                            {entry.reference_id && (
                              <span className="block text-[10px] text-neutral-500 font-mono">
                                Ref: {entry.reference_id}
                              </span>
                            )}
                          </td>
                          <td className="py-3.5 px-4 text-right whitespace-nowrap font-mono font-bold text-sm">
                            <span
                              className={isPositive ? "text-amber-400" : "text-neutral-400"}
                            >
                              {isPositive ? `+${entry.amount}` : entry.amount}
                            </span>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            ) : (
              <div className="p-8 text-center text-xs text-neutral-500">
                No ledger transactions recorded yet.
              </div>
            )}
          </div>
        ) : (
          /* Payment Orders Table */
          <div className="p-6 rounded-2xl bg-[#0e1017]/75 backdrop-blur-2xl border border-white/[0.08] space-y-4 shadow-[inset_0_1px_1px_rgba(255,255,255,0.08),0_12px_32px_rgba(0,0,0,0.35)]">
            <h2 className="text-base font-bold text-white flex items-center gap-2">
              <History className="h-4 w-4 text-amber-400" />
              Razorpay Payment Orders
            </h2>

            {isLoading ? (
              <div className="space-y-3 animate-pulse">
                {[1, 2, 3, 4].map((i) => (
                  <div key={i} className="h-14 bg-white/[0.04] rounded-xl border border-white/[0.06]" />
                ))}
              </div>
            ) : paymentOrders.length > 0 ? (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs border-collapse">
                  <thead>
                    <tr className="border-b border-white/[0.08] text-neutral-400 font-mono uppercase text-[11px]">
                      <th className="py-3 px-4">Date / Time</th>
                      <th className="py-3 px-4">Order ID</th>
                      <th className="py-3 px-4">Package</th>
                      <th className="py-3 px-4">Price (Paise)</th>
                      <th className="py-3 px-4">Credits</th>
                      <th className="py-3 px-4 text-right">Status</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-white/[0.04]">
                    {paymentOrders.map((ord) => (
                      <tr key={ord.id} className="hover:bg-white/[0.03] transition-colors">
                        <td className="py-3.5 px-4 font-mono text-neutral-400 whitespace-nowrap">
                          {new Date(ord.created_at).toLocaleString()}
                        </td>
                        <td className="py-3.5 px-4 font-mono text-neutral-300">
                          <span className="text-[11px] font-semibold">{ord.id.slice(0, 8)}...</span>
                          {ord.razorpay_order_id && (
                            <span className="block text-[10px] text-neutral-500">
                              RP: {ord.razorpay_order_id}
                            </span>
                          )}
                        </td>
                        <td className="py-3.5 px-4 text-neutral-200 font-medium">
                          {ord.package_id === "pkg_starter"
                            ? "Starter Pack"
                            : ord.package_id === "pkg_pro"
                            ? "Pro Pack"
                            : ord.package_id === "pkg_master"
                            ? "Master Pack"
                            : ord.package_id}
                        </td>
                        <td className="py-3.5 px-4 font-mono font-bold text-white">
                          ₹{(ord.amount_paise / 100).toFixed(2)}
                        </td>
                        <td className="py-3.5 px-4 font-mono text-amber-400 font-bold">
                          +{ord.credits_amount} cr
                        </td>
                        <td className="py-3.5 px-4 text-right whitespace-nowrap">
                          {getOrderStatusBadge(ord.status)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <div className="p-8 text-center text-xs text-neutral-500">
                No payment orders created yet.
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
