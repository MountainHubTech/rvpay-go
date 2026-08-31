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

// Maps the incoming iframe event payload directly to your paymentContext
const paymentContext = useMemo(() => {
  return {
    type: initiateProps?.type ?? null,
    transactionId: initiateProps?.transactionId ?? null,
    apiKey: initiateProps?.publishableKey ?? null, // Maps your publishableKey property to the old apiKey field
    chargeId: null, // Note: Not natively present in PaymentInitiateProps; explicitly null
    subscriptionId: initiateProps?.subscriptionId ?? null,
    amount: initiateProps?.amount ?? null, // Retains number type or null
  };
}, [initiateProps]);

  // Effective payment context: the HighLevel payment_initiate_props event is
  // the primary source; URL parameters remain a fallback.
  const transactionId =
    initiateProps?.transactionId ?? paymentContext.transactionId;
  const subscriptionId =
    initiateProps?.subscriptionId ?? paymentContext.subscriptionId;

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

  async function queryPaymentStatus(): Promise<QueryResponse> {
    const response = await fetch(
      "https://api.rvpay.xyz/payments/custom-provider/query",
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          // The queryUrl verify contract is `type=verify`. HighLevel does not
          // pass `type` on the paymentsUrl, so set the documented value here.
          type: paymentContext.type ?? "verify",
          transactionId: transactionId,
          orderId: initiateProps?.orderId,
          // The provider API key is normally a server-side credential sent to
          // queryUrl; only forward it when HighLevel actually supplied one.
          apiKey: paymentContext.apiKey,
          chargeId: paymentContext.chargeId,
          subscriptionId: subscriptionId,
        }),
      }
    );

    if (!response.ok) {
      throw new Error("Unable to verify payment status.");
    }

    return response.json();
  }

  function stopPolling() {
    if (pollingRef.current) {
      window.clearInterval(pollingRef.current);
      pollingRef.current = null;
    }
  }

  async function handlePay() {
    if (!agreed || !phone || isSubmitting) {
      return;
    }

    // HighLevel delivers the transaction context via the payment_initiate_props
    // postMessage event (transactionId required); URL params are the fallback.
    // The provider API key is a server-side credential delivered to queryUrl,
    // so it is not required here; apiKey/chargeId are forwarded when present.
    if (!transactionId) {
      setStatusMessage(
        "Invalid payment request. Missing payment transaction information."
      );
      return;
    }

    setIsSubmitting(true);
    setStatusMessage("Verifying payment request...");

    try {
      const initialStatus = await queryPaymentStatus();

      if (initialStatus.failed) {
        stopPolling();
        setStatusMessage(
          initialStatus.message ?? "This payment request has failed."
        );
        setIsSubmitting(false);
        return;
      }

      if (initialStatus.success === false) {
        stopPolling();
        setStatusMessage(
          initialStatus.message ?? "This payment request is not available."
        );
        setIsSubmitting(false);
        return;
      }

      setStatusMessage(
        "Payment request verified. Processing your mobile money payment..."
      );

      /*
       * The payment request itself must be created through the RVPay
       * public Clients/Transactions service flow. This page receives the
       * transaction context from the payment URL and verifies it through:
       *
       * POST https://api.rvpay.xyz/payments/custom-provider/query
       *
       * The query service correlates the external transaction with the
       * RVPay deposit/transaction.
       *
       * Payment initiation should use the existing public payment/deposit
       * endpoint exposed by the Transactions service in your backend flow.
       */

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
              status.message ?? "The payment could not be completed."
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
