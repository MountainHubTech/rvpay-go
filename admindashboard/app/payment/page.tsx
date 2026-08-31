"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { cn } from "@/lib/utils";

type Provider = {
  id: string;
  name: string;
  bg: string;
  fg: string;
  shape?: "pill" | "square";
};

// Static demo data — countries + mobile money providers supported by pawaPay.
// Source: pawaPay merchant docs (docs.pawapay.io/v2/docs/providers).
const PROVIDERS: Record<string, Provider> = {
  mtn: { id: "mtn", name: "MTN", bg: "#FFCC08", fg: "#111111", shape: "pill" },
  orange: { id: "orange", name: "Orange Money", bg: "#000000", fg: "#FF7900", shape: "square" },
  moov: { id: "moov", name: "Moov Money", bg: "#0033A0", fg: "#FFFFFF", shape: "square" },
  wave: { id: "wave", name: "Wave", bg: "#1DA3E0", fg: "#FFFFFF", shape: "square" },
  airtel: { id: "airtel", name: "Airtel Money", bg: "#ED1C24", fg: "#FFFFFF", shape: "square" },
  vodacom: { id: "vodacom", name: "Vodacom M-Pesa", bg: "#E60000", fg: "#FFFFFF", shape: "square" },
  mpesa: { id: "mpesa", name: "M-Pesa", bg: "#43B02A", fg: "#FFFFFF", shape: "square" },
  at: { id: "at", name: "AirtelTigo", bg: "#0057B8", fg: "#FFFFFF", shape: "square" },
  vodafone: { id: "vodafone", name: "Vodafone Cash", bg: "#E60000", fg: "#FFFFFF", shape: "square" },
  tnm: { id: "tnm", name: "TNM Mpamba", bg: "#00A651", fg: "#FFFFFF", shape: "square" },
  movitel: { id: "movitel", name: "Movitel", bg: "#009540", fg: "#FFFFFF", shape: "square" },
  free: { id: "free", name: "Free Money", bg: "#E2001A", fg: "#FFFFFF", shape: "square" },
  zamtel: { id: "zamtel", name: "Zamtel Kwacha", bg: "#00A651", fg: "#FFFFFF", shape: "square" },
  tigo: { id: "tigo", name: "Tigo Pesa", bg: "#0072CE", fg: "#FFFFFF", shape: "square" },
  halotel: { id: "halotel", name: "Halotel", bg: "#FFC20E", fg: "#111111", shape: "square" },
  safaricom: { id: "safaricom", name: "Safaricom M-Pesa", bg: "#43B02A", fg: "#FFFFFF", shape: "square" },
};

type Country = {
  code: string;
  name: string;
  flag: string;
  dialCode: string;
  currency: string;
  providers: string[];
};

const COUNTRIES: Country[] = [
  { code: "BJ", name: "Benin", flag: "🇧🇯", dialCode: "+229", currency: "FCFA", providers: ["mtn", "moov"] },
  { code: "BF", name: "Burkina Faso", flag: "🇧🇫", dialCode: "+226", currency: "FCFA", providers: ["moov", "orange"] },
  { code: "CM", name: "Cameroon", flag: "🇨🇲", dialCode: "+237", currency: "FCFA", providers: ["mtn", "orange"] },
  { code: "CI", name: "Côte d'Ivoire", flag: "🇨🇮", dialCode: "+225", currency: "FCFA", providers: ["mtn", "orange", "wave"] },
  { code: "CD", name: "DR Congo", flag: "🇨🇩", dialCode: "+243", currency: "CDF", providers: ["vodacom", "airtel", "orange"] },
  { code: "ET", name: "Ethiopia", flag: "🇪🇹", dialCode: "+251", currency: "ETB", providers: ["safaricom"] },
  { code: "GA", name: "Gabon", flag: "🇬🇦", dialCode: "+241", currency: "FCFA", providers: ["airtel"] },
  { code: "GH", name: "Ghana", flag: "🇬🇭", dialCode: "+233", currency: "GHS", providers: ["mtn", "at", "vodafone"] },
  { code: "KE", name: "Kenya", flag: "🇰🇪", dialCode: "+254", currency: "KES", providers: ["mpesa"] },
  { code: "LS", name: "Lesotho", flag: "🇱🇸", dialCode: "+266", currency: "LSL", providers: ["mpesa"] },
  { code: "MW", name: "Malawi", flag: "🇲🇼", dialCode: "+265", currency: "MWK", providers: ["airtel", "tnm"] },
  { code: "MZ", name: "Mozambique", flag: "🇲🇿", dialCode: "+258", currency: "MZN", providers: ["movitel", "vodacom"] },
  { code: "NG", name: "Nigeria", flag: "🇳🇬", dialCode: "+234", currency: "NGN", providers: ["airtel", "mtn"] },
  { code: "CG", name: "Republic of Congo", flag: "🇨🇬", dialCode: "+242", currency: "FCFA", providers: ["airtel", "mtn"] },
  { code: "RW", name: "Rwanda", flag: "🇷🇼", dialCode: "+250", currency: "RWF", providers: ["airtel", "mtn"] },
  { code: "SN", name: "Senegal", flag: "🇸🇳", dialCode: "+221", currency: "FCFA", providers: ["free", "orange", "wave"] },
  { code: "SL", name: "Sierra Leone", flag: "🇸🇱", dialCode: "+232", currency: "SLE", providers: ["orange"] },
  { code: "TZ", name: "Tanzania", flag: "🇹🇿", dialCode: "+255", currency: "TZS", providers: ["airtel", "vodacom", "tigo", "halotel"] },
  { code: "UG", name: "Uganda", flag: "🇺🇬", dialCode: "+256", currency: "UGX", providers: ["airtel", "mtn"] },
  { code: "ZM", name: "Zambia", flag: "🇿🇲", dialCode: "+260", currency: "ZMW", providers: ["airtel", "mtn", "zamtel"] },
];

type QueryResponse = {
  success?: boolean;
  failed?: boolean;
  message?: string;
};

// HighLevel Custom Payment Provider "payment_initiate_props" event. HighLevel
// posts this into the paymentsUrl iframe (after the iframe sends a "ready"
// event) instead of putting the payment context on the URL.
type PaymentInitiateProps = {
  type: string;
  publishableKey?: string;
  amount?: number;
  currency?: string;
  mode?: string;
  productDetails?: Array<Record<string, unknown>>;
  contact?: {
    id?: string;
    name?: string;
    email?: string;
    contact?: string;
    shippingAddress?: Record<string, unknown>;
  };
  orderId?: string;
  transactionId?: string;
  subscriptionId?: string;
  locationId?: string;
};

// Unambiguous ISO currency → country mapping used to preselect the checkout
// country. FCFA currencies (XOF/XAF) span multiple supported countries and are
// deliberately not mapped.
const CURRENCY_COUNTRY: Record<string, string> = {
  KES: "KE",
  GHS: "GH",
  NGN: "NG",
  TZS: "TZ",
  UGX: "UG",
  ZMW: "ZM",
  RWF: "RW",
  MZN: "MZ",
  MWK: "MW",
  SLE: "SL",
  ETB: "ET",
  CDF: "CD",
  LSL: "LS",
};


function ProviderLogo({ provider }: { provider: Provider }) {
  if (provider.shape === "pill") {
    return (
      <span
        className="rounded-full px-4 py-1.5 text-xs font-extrabold tracking-wide"
        style={{ backgroundColor: provider.bg, color: provider.fg }}
      >
        {provider.name}
      </span>
    );
  }
  return (
    <div
      className="flex h-full w-full flex-col items-center justify-center gap-0.5 rounded-md text-center leading-tight"
      style={{ backgroundColor: provider.bg, color: provider.fg }}
    >
      <span className="text-[10px] font-bold">
        {provider.name.split(" ").map((word, i) => (
          <span key={i} className="block">
            {word}
          </span>
        ))}
      </span>
    </div>
  );
}

export default function PaymentPage() {
  const [countryCode, setCountryCode] = useState("CM");
  const [countryOpen, setCountryOpen] = useState(false);
  const [amount, setAmount] = useState(5000);
  const [editingAmount, setEditingAmount] = useState(false);
  const [phone, setPhone] = useState("");
  const [agreed, setAgreed] = useState(false);

  const country = useMemo(
    () => COUNTRIES.find((c) => c.code === countryCode)!,
    [countryCode]
  );

  const [providerId, setProviderId] = useState(country.providers[0]);
  const selectedProvider = PROVIDERS[providerId];

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [statusMessage, setStatusMessage] = useState<string | null>(null);
  const pollingRef = useRef<number | null>(null);
  const [initiateProps, setInitiateProps] =
    useState<PaymentInitiateProps | null>(null);

  // HighLevel iframe handshake: announce readiness so HighLevel sends the
  // payment_initiate_props event, then capture it. URL params remain a
  // fallback for direct/manual loads.
useEffect(() => {
  console.log("========================================");
  console.log("[RVPay] Payment iframe initialized");
  console.log("[RVPay] window === window.parent:", window === window.parent);
  console.log("[RVPay] window.parent:", window.parent);
  console.log("[RVPay] current origin:", window.location.origin);
  console.log("[RVPay] parent origin:", document.referrer);

  function onMessage(event: MessageEvent) {
    console.log("========================================");
    console.log("[RVPay] MESSAGE RECEIVED");
    console.log("[RVPay] event.origin:", event.origin);
    console.log("[RVPay] event.source === window.parent:", event.source === window.parent);
    console.log("[RVPay] event.data:", event.data);

    const data = event.data;

    if (!data || typeof data === "string") {
      return;
    }

    if (data.type === "payment_initiate_props") {
      console.log("[RVPay] payment_initiate_props RECEIVED!");
      console.log("[RVPay] transactionId:", data.transactionId);
      console.log("[RVPay] orderId:", data.orderId);
      console.log("[RVPay] amount:", data.amount);
      console.log("[RVPay] currency:", data.currency);
      console.log("[RVPay] locationId:", data.locationId);

      setInitiateProps(data);
    }
  }

  window.addEventListener("message", onMessage);

  const readyMessage = {
    type: "custom_provider_ready",
    loaded: true,
  };

  console.log("[RVPay] Sending ready event:", readyMessage);
  console.log("[RVPay] Sending to parent:", window.parent);

  window.parent.postMessage(readyMessage, "*");

  console.log("[RVPay] Ready event sent");

  return () => {
    console.log("[RVPay] Removing message listener");
    window.removeEventListener("message", onMessage);
  };
}, []);

// SUSPENDED: the HighLevel payment_initiate_props handshake is currently
// NON-AUTHORITATIVE. Live testing (see the [RVPay] console diagnostics in the
// effect above) proves HighLevel never sends payment_initiate_props after the
// custom_provider_ready post, so nothing in the payment flow may depend on it.
// The authoritative transactionId is read from the payment URL below. The
// listener/diagnostics are retained for future reactivation only.
const paymentContext = useMemo(() => {
  if (typeof window === "undefined") {
    return {
      type: null,
      transactionId: null,
      apiKey: null,
      chargeId: null,
      subscriptionId: null,
      amount: null,
    };
  }

  const params = new URLSearchParams(window.location.search);

  // NOTE: buyNowProductId is NOT a transactionId and is deliberately not read
  // here. If the URL carries a genuine HighLevel transactionId it is used;
  // otherwise the transactionId is represented as unavailable (null).
  return {
    type: params.get("type"),
    transactionId: params.get("transactionId"),
    apiKey: params.get("apiKey"),
    chargeId: params.get("chargeId"),
    subscriptionId: params.get("subscriptionId"),
    amount: params.get("amount"),
  };
}, []);

// The GHL transactionId, when a REAL one is available (URL param or the
// suspended payment_initiate_props event). It is used ONLY for GHL-specific
// status verification — never as a gate in front of payment initiation, and
// never fabricated when unavailable.
const transactionId =
  paymentContext.transactionId ?? initiateProps?.transactionId ?? null;
const subscriptionId =
  paymentContext.subscriptionId ?? initiateProps?.subscriptionId ?? null;
const chargeId = paymentContext.chargeId;

  // Order prefill from the HighLevel payment_initiate_props event: amount,
  // currency → country preselection (only for unambiguous currencies), and
  // the customer's phone number (dial code stripped). Falls back to the
  // `amount` URL parameter when no event payload arrived.
  useEffect(() => {
    const props = initiateProps;
    if (!props) {
      if (!paymentContext.amount) return;
      const parsed = Number(paymentContext.amount);
      if (Number.isFinite(parsed) && parsed > 0) {
        setAmount(parsed);
        setEditingAmount(false);
      }
      return;
    }

    if (typeof props.amount === "number" && props.amount > 0) {
      setAmount(props.amount);
      setEditingAmount(false);
    }

    const isoCurrency = (props.currency ?? "").toUpperCase();
    const mappedCountry = CURRENCY_COUNTRY[isoCurrency];
    const targetCode = mappedCountry ?? countryCode;
    const target = COUNTRIES.find((c) => c.code === targetCode);
    if (target && targetCode !== countryCode) {
      setCountryCode(targetCode);
      setProviderId(target.providers[0]);
      setCountryOpen(false);
    }

    const rawPhone = (props.contact?.contact ?? "").replace(/\D/g, "");
    if (rawPhone && target) {
      const dialDigits = target.dialCode.replace(/\D/g, "");
      setPhone(
        rawPhone.startsWith(dialDigits)
          ? rawPhone.slice(dialDigits.length)
          : rawPhone
      );
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initiateProps, paymentContext.amount]);


  useEffect(() => {
    return () => {
      if (pollingRef.current) {
        window.clearInterval(pollingRef.current);
      }
    };
  }, []);

  function handleCountrySelect(code: string) {
    const next = COUNTRIES.find((c) => c.code === code)!;
    setCountryCode(code);
    setProviderId(next.providers[0]);
    setPhone("");
    setCountryOpen(false);
  }

  // Existing Transactions-service verify endpoint (grpc-gateway):
  // GET /v1/public/payments/verify (VerifyPayment contract: success/failed).
  async function queryPaymentStatus(): Promise<QueryResponse> {
    const params = new URLSearchParams();
    if (transactionId) {
      params.set("ghlTransactionId", transactionId);
    }
    if (chargeId) {
      params.set("ghlChargeId", chargeId);
    }
    if (subscriptionId) {
      params.set("subscriptionId", subscriptionId);
    }

    const response = await fetch(
      `https://api.rvpay.xyz/v1/public/payments/verify?${params.toString()}`,
      { method: "GET" }
    );

    if (!response.ok) {
      throw new Error("Unable to verify payment status.");
    }

    return response.json();
  }

  // Map the UI provider selection to the existing Transactions-service
  // contract (commongrpc.Provider). The backend currently only supports MTN
  // and Orange mobile money; unsupported selections are rejected up front
  // instead of sending a fabricated provider value.
  function providerToApi(id: string): string | null {
    switch (id) {
      case "mtn":
        return "PROVIDER_MTN_MOMO";
      case "orange":
        return "PROVIDER_ORANGE_MOMO";
      default:
        return null;
    }
  }

  // The backend Money contract requires an ISO-4217 code. The UI groups the
  // CFA franc as "FCFA"; the repository's documented convention uses XAF.
  function currencyToApi(currency: string): string {
    return currency === "FCFA" ? "XAF" : currency.toUpperCase();
  }

  // Initiate the payment through the EXISTING Transactions-service endpoint:
  // POST /v1/public/deposits (CreateDepositRequest contract). Only real values
  // from the existing flow are sent; client/customer/merchant identifiers are
  // forwarded when HighLevel/RVPay supplied them on the payment URL, and no
  // identifiers are ever invented on the frontend. Returns the legitimate
  // deposit identifier from the backend response when one is provided.
  async function initiatePayment(): Promise<string | null> {
    const provider = providerToApi(providerId);
    if (!provider) {
      throw new Error(
        "This mobile money provider is not supported yet. Please select MTN or Orange Money."
      );
    }

    const urlParams = new URLSearchParams(window.location.search);
    const clientId = urlParams.get("clientId") ?? "";
    const customerId = urlParams.get("customerId") ?? "";
    const merchantId = urlParams.get("merchantId") ?? "";

    const body: Record<string, unknown> = {
      amount: {
        amount: amount.toFixed(2),
        currency: currencyToApi(country.currency),
      },
      paymentType: "PAYMENT_TYPE_MMO",
      payerPhoneNumber: `${country.dialCode}${phone}`,
      provider,
    };
    if (clientId) {
      body.clientId = clientId;
    }
    if (customerId) {
      body.customerId = customerId;
    }
    if (merchantId) {
      body.merchantId = merchantId;
    }

    const response = await fetch("https://api.rvpay.xyz/v1/public/deposits", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(body),
    });

    if (!response.ok) {
      // Surface the backend's own error honestly when it provides one.
      let detail: string | null = null;
      try {
        const text = await response.text();
        if (text) {
          detail = text;
        }
      } catch {
        // ignore body read failures; fall back to the generic message
      }
      throw new Error(
        detail ??
          "Unable to initiate the payment. Please try again."
      );
    }

    // Retain the legitimate deposit identifier if the backend returned one.
    // HTTP 200 alone is NOT treated as payment success.
    try {
      const data = (await response.json()) as {
        deposit?: { id?: string } | null;
      };
      return data?.deposit?.id ?? null;
    } catch {
      return null;
    }
  }

  function stopPolling() {
    if (pollingRef.current) {
      window.clearInterval(pollingRef.current);
      pollingRef.current = null;
    }
  }

  async function handlePay() {
    // User-facing form validation: terms, phone, provider, and a positive
    // amount. Payment initiation does NOT depend on a GHL transactionId —
    // live testing shows the iframe URL does not carry one and HighLevel does
    // not send payment_initiate_props, so blocking on it would permanently
    // prevent the payment request from reaching the backend.
    if (!agreed || !phone || isSubmitting) {
      return;
    }

    if (!Number.isFinite(amount) || amount <= 0) {
      setStatusMessage("Please enter a valid payment amount.");
      return;
    }

    setIsSubmitting(true);
    setStatusMessage("Initiating payment...");

    try {
      // Initiate the payment through the existing RVPay backend. The backend
      // remains authoritative for payment state; the frontend only initiates
      // and displays it. isSubmitting prevents duplicate submissions while
      // one is processing.
      const depositId = await initiatePayment();
      if (depositId) {
        console.log("[RVPay] Deposit initiated:", depositId);
      }

      // GHL-specific verification only when a REAL GHL transactionId exists.
      // Never fabricate one and never claim GHL verification occurred
      // without it.
      if (!transactionId) {
        setStatusMessage(
          "Payment request submitted. The deposit was accepted by RVPay; transaction status tracking is unavailable until the payment provider correlation is configured."
        );
        setIsSubmitting(false);
        return;
      }

      // Poll the existing status endpoint with the same transactionId until
      // success/failure. A single interval is used (stopPolling first).
      setStatusMessage("Waiting for mobile money confirmation...");

      stopPolling();

      pollingRef.current = window.setInterval(async () => {
        try {
          const status = await queryPaymentStatus();

          if (status.success === true) {
            stopPolling();
            setStatusMessage("Payment completed successfully.");
            setIsSubmitting(false);
            return;
          }

          if (status.failed || status.success === false) {
            stopPolling();
            setStatusMessage(
              status.message ?? "Payment failed."
            );
            setIsSubmitting(false);
          }
        } catch {
          stopPolling();
          setStatusMessage("Unable to verify payment status.");
          setIsSubmitting(false);
        }
      }, 3000);
    } catch (error) {
      stopPolling();
      setStatusMessage(
        error instanceof Error
          ? error.message
          : "Unable to process the payment."
      );
      setIsSubmitting(false);
    }
  }

  return (
    <div className="flex min-h-screen flex-col items-center bg-[#eceff2] px-4 py-16">
      <h1 className="mb-6 text-base font-semibold text-foreground">
        Make payment
      </h1>

      <div className="w-full max-w-md space-y-4">
        <div className="rounded-xl bg-white p-5 shadow-sm">
          {/* Payment amount */}
          <div>
            <label className="text-xs text-muted-foreground">
              Payment amount
            </label>
            <div className="mt-1 flex items-center justify-between">
              {editingAmount ? (
                <input
                  autoFocus
                  type="number"
                  min={0}
                  value={amount}
                  onChange={(e) => setAmount(Number(e.target.value))}
                  onBlur={() => setEditingAmount(false)}
                  onKeyDown={(e) => e.key === "Enter" && setEditingAmount(false)}
                  className="w-32 border-b border-primary text-lg font-semibold text-primary outline-none"
                />
              ) : (
                <span className="text-lg font-semibold text-primary">
                  {amount.toLocaleString()}
                  {country.currency}
                </span>
              )}
              <button
                type="button"
                aria-label="Edit amount"
                onClick={() => setEditingAmount((v) => !v)}
                className="text-muted-foreground hover:text-foreground"
              >
                ✎
              </button>
            </div>
          </div>

          {/* Country */}
          <div className="relative mt-4">
            <label className="text-xs text-muted-foreground">Country</label>
            <button
              type="button"
              onClick={() => setCountryOpen((v) => !v)}
              className="mt-1 flex w-full items-center justify-between rounded-md bg-muted px-3 py-2.5 text-left text-sm"
            >
              <span className="flex items-center gap-2">
                <span>{country.flag}</span>
                <span>{country.name}</span>
              </span>
              <span className="text-muted-foreground">⌄</span>
            </button>

            {countryOpen && (
              <div className="absolute z-10 mt-1 max-h-64 w-full overflow-auto rounded-md border border-border bg-white shadow-md">
                {COUNTRIES.map((c) => (
                  <button
                    key={c.code}
                    type="button"
                    onClick={() => handleCountrySelect(c.code)}
                    className={cn(
                      "flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-muted",
                      c.code === countryCode && "bg-muted"
                    )}
                  >
                    <span>{c.flag}</span>
                    <span>{c.name}</span>
                  </button>
                ))}
              </div>
            )}
          </div>

          {/* Mobile money number */}
          <div className="mt-4">
            <label className="text-xs text-muted-foreground">
              {selectedProvider?.name ?? "Mobile money"} number
            </label>
            <div className="mt-1 flex items-center gap-2 rounded-md bg-muted px-3 py-2.5">
              <span className="text-muted-foreground">⌄</span>
              <span className="text-sm text-muted-foreground">
                {country.dialCode}
              </span>
              <input
                type="tel"
                value={phone}
                onChange={(e) => setPhone(e.target.value.replace(/[^0-9]/g, ""))}
                placeholder=""
                className="w-full bg-transparent text-sm outline-none"
              />
            </div>
          </div>

          {/* Payment method */}
          <div className="mt-4">
            <label className="text-xs text-muted-foreground">
              Select payment method
            </label>
            <div className="mt-2 flex flex-wrap gap-3">
              {country.providers.map((id) => {
                const provider = PROVIDERS[id];
                const selected = id === providerId;
                return (
                  <button
                    key={id}
                    type="button"
                    onClick={() => setProviderId(id)}
                    className={cn(
                      "flex h-16 w-16 items-center justify-center overflow-hidden rounded-lg border-2 p-1.5",
                      selected
                        ? "border-primary"
                        : "border-border"
                    )}
                  >
                    <ProviderLogo provider={provider} />
                  </button>
                );
              })}
            </div>
          </div>

          {/* Terms */}
          <div className="mt-4 flex items-center gap-2">
            <input
              id="terms"
              type="checkbox"
              checked={agreed}
              onChange={(e) => setAgreed(e.target.checked)}
              className="h-3.5 w-3.5 rounded border-border"
            />
            <label htmlFor="terms" className="text-xs text-muted-foreground">
              I agree to RVPay&apos;s{" "}
              <a href="#" className="text-primary underline">
                Terms &amp; conditions
              </a>
            </label>
          </div>

          {statusMessage && (
            <p className="mt-4 text-xs text-muted-foreground">
              {statusMessage}
            </p>
          )}
        </div>

        <button
          type="button"
          disabled={!agreed || !phone || isSubmitting}
          onClick={handlePay}
          className="w-full rounded-md bg-[#1a237e] py-3 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-50"
        >
          {isSubmitting
            ? "Processing..."
            : `Pay ${amount.toLocaleString()}${country.currency}`}
        </button>
      </div>
    </div>
  );
}
