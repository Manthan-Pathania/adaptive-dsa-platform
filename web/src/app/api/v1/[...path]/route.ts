import { NextRequest, NextResponse } from "next/server";

function getBackendBase(): string {
  if (process.env.BACKEND_API_URL) {
    return process.env.BACKEND_API_URL;
  }
  // If running inside docker container, backend service is reachable at http://backend:8080
  // If running on host, backend is at http://127.0.0.1:8080
  return process.env.NODE_ENV === "production" ? "http://backend:8080" : "http://127.0.0.1:8080";
}

async function proxyRequest(req: NextRequest, { params }: { params: Promise<{ path: string[] }> }) {
  const { path } = await params;
  const backendBase = getBackendBase();
  const subPath = path ? path.join("/") : "";
  const targetUrl = new URL(`/api/v1/${subPath}${req.nextUrl.search}`, backendBase);

  const forwardHeaders = new Headers();
  req.headers.forEach((value, key) => {
    const lowerKey = key.toLowerCase();
    // Strip host and content-length so fetch calculates accurate byte length
    if (lowerKey !== "host" && lowerKey !== "content-length") {
      forwardHeaders.set(key, value);
    }
  });

  const method = req.method.toUpperCase();
  let body: BodyInit | undefined = undefined;
  if (method !== "GET" && method !== "HEAD") {
    const arrayBuffer = await req.arrayBuffer();
    if (arrayBuffer.byteLength > 0) {
      body = Buffer.from(arrayBuffer);
    }
  }

  try {
    const backendRes = await fetch(targetUrl.toString(), {
      method,
      headers: forwardHeaders,
      body,
      redirect: "manual",
      cache: "no-store",
    });

    const responseHeaders = new Headers();
    backendRes.headers.forEach((value, key) => {
      if (key.toLowerCase() !== "set-cookie") {
        responseHeaders.set(key, value);
      }
    });

    // Use getSetCookie() to preserve multiple Set-Cookie headers (e.g. session_token AND csrf_token)
    if (typeof backendRes.headers.getSetCookie === "function") {
      const setCookies = backendRes.headers.getSetCookie();
      for (const cookie of setCookies) {
        responseHeaders.append("set-cookie", cookie);
      }
    } else {
      const rawCookie = backendRes.headers.get("set-cookie");
      if (rawCookie) {
        responseHeaders.set("set-cookie", rawCookie);
      }
    }

    return new NextResponse(backendRes.body, {
      status: backendRes.status,
      statusText: backendRes.statusText,
      headers: responseHeaders,
    });
  } catch (err: unknown) {
    const e = err as Error;
    return NextResponse.json(
      {
        error: "Backend service unreachable",
        detail: e.message || "Failed to proxy request to backend service",
        target: targetUrl.toString(),
      },
      { status: 502 }
    );
  }
}

export const GET = proxyRequest;
export const POST = proxyRequest;
export const PUT = proxyRequest;
export const PATCH = proxyRequest;
export const DELETE = proxyRequest;
