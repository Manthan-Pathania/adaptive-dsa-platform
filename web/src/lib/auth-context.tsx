"use client";

import React, { createContext, useContext, useEffect, useState, useCallback } from "react";
import { User } from "@/types";
import { api } from "@/lib/api";

interface AuthContextType {
  user: User | null;
  credits: number;
  loading: boolean;
  login: (email: string, pass: string) => Promise<void>;
  register: (email: string, pass: string, name: string) => Promise<void>;
  logout: () => Promise<void>;
  refreshUser: () => Promise<void>;
  refreshCredits: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [credits, setCredits] = useState<number>(0);
  const [loading, setLoading] = useState(true);

  const refreshCredits = useCallback(async () => {
    try {
      const data = await api.getCreditBalance();
      setCredits(data.balance);
    } catch {
      // Ignored if unauthenticated
    }
  }, []);

  const refreshUser = useCallback(async () => {
    try {
      const data = await api.me();
      setUser(data.user);
      await refreshCredits();
    } catch {
      setUser(null);
    } finally {
      setLoading(false);
    }
  }, [refreshCredits]);

  useEffect(() => {
    refreshUser();
  }, [refreshUser]);

  const login = async (email: string, pass: string) => {
    const data = await api.login(email, pass);
    setUser(data.user);
    await refreshCredits();
  };

  const register = async (email: string, pass: string, name: string) => {
    const data = await api.register(email, pass, name);
    setUser(data.user);
    await refreshCredits();
  };

  const logout = async () => {
    try {
      await api.logout();
    } finally {
      setUser(null);
      setCredits(0);
    }
  };

  return (
    <AuthContext.Provider
      value={{
        user,
        credits,
        loading,
        login,
        register,
        logout,
        refreshUser,
        refreshCredits,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}
