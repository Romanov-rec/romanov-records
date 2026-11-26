"use client";

import { useEffect, useState, useSyncExternalStore } from "react";
import { createPortal } from "react-dom";
import { fetchPaymentStatus, type PaymentStatus } from "./api";
import { formatRub, TEAL } from "./config";

type View = PaymentStatus | "checking" | "error";

const POLL_MS = 3000;
const POLL_LIMIT = 40; // ~2 минуты

// Клиент вернулся с ЮKassa на /?payment=<id>: показываем, прошла ли оплата
const noSubscribe = () => () => {};
const readPaymentParam = () => new URLSearchParams(window.location.search).get("payment") ?? "";

export default function PaymentReturn() {
  // На сервере параметра нет — модалка появляется только в браузере, без расхождения при гидратации
  const raw = useSyncExternalStore(noSubscribe, readPaymentParam, () => "");
  const [dismissed, setDismissed] = useState(false);
  const [view, setView] = useState<View>("checking");
  const [amount, setAmount] = useState(0);
  const parsed = Number.parseInt(raw, 10);
  const id = !dismissed && Number.isFinite(parsed) && parsed > 0 ? parsed : null;

  useEffect(() => {
    if (id === null) return;
    let stopped = false;
    let attempts = 0;
    let timer: ReturnType<typeof setTimeout> | undefined;

    const check = async () => {
      attempts += 1;
      try {
        const res = await fetchPaymentStatus(id);
        if (stopped) return;
        setAmount(res.amount);
        setView(res.status === "pending" ? "pending" : res.status);
        if (res.status === "pending" && attempts < POLL_LIMIT) timer = setTimeout(check, POLL_MS);
      } catch {
        if (stopped) return;
        if (attempts < 3) timer = setTimeout(check, POLL_MS);
        else setView("error");
      }
    };
    check();
    return () => { stopped = true; if (timer) clearTimeout(timer); };
  }, [id]);

  if (id === null) return null;

  const close = () => {
    const url = new URL(window.location.href);
    url.searchParams.delete("payment");
    window.history.replaceState(null, "", url.pathname + url.search + url.hash);
    setDismissed(true);
  };

  const text: Record<View, { title: string; body: string }> = {
    checking: { title: "Проверяем оплату", body: "Это займёт несколько секунд" },
    pending: {
      title: "Ждём подтверждение оплаты",
      body: "Если вы уже заплатили, статус обновится сам. Если закрыли страницу оплаты, не заплатив, бронь снимется через 30 минут",
    },
    paid: {
      title: "Оплата прошла",
      body: `${amount ? formatRub(amount) + " получили, время" : "Время"} за вами. Чек придёт по SMS, мы свяжемся, чтобы обсудить детали`,
    },
    expired: { title: "Оплата не прошла", body: "Время освободили. Можно записаться ещё раз" },
    none: { title: "Запись принята", body: "Мы свяжемся с вами для подтверждения" },
    error: {
      title: "Не получилось проверить оплату",
      body: "Если деньги списались, напишите или позвоните нам — всё решим",
    },
  };
  const t = text[view];
  const ok = view === "paid" || view === "none";
  const busy = view === "checking" || view === "pending";

  return createPortal(
    <div
      onClick={busy ? undefined : close}
      style={{ position: "fixed", inset: 0, zIndex: 9100, background: "rgba(5,37,33,0.6)", display: "flex", alignItems: "center", justifyContent: "center", padding: "1rem" }}
    >
      <div
        role="status"
        aria-live="polite"
        onClick={e => e.stopPropagation()}
        style={{ background: "rgba(7,56,53,0.98)", border: "1px solid rgba(255,255,255,0.1)", borderRadius: 24, padding: "2rem", width: "100%", maxWidth: 420, textAlign: "center" }}
      >
        <div style={{ width: 56, height: 56, borderRadius: "50%", margin: "0 auto 1.1rem", display: "flex", alignItems: "center", justifyContent: "center",
          border: `2px solid ${ok ? TEAL : busy ? "rgba(255,255,255,0.25)" : "#f87171"}` }}>
          {ok ? (
            <svg aria-hidden="true" width="24" height="18" viewBox="0 0 28 22" fill="none"><path d="M2 11L10 19L26 3" stroke={TEAL} strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" /></svg>
          ) : busy ? (
            <span aria-hidden="true" className="payment-return-spinner" />
          ) : (
            <svg aria-hidden="true" width="18" height="18" viewBox="0 0 18 18" fill="none"><path d="M3 3L15 15M15 3L3 15" stroke="#f87171" strokeWidth="2.2" strokeLinecap="round" /></svg>
          )}
        </div>
        <h2 style={{ fontFamily: '"Borsok", sans-serif', color: "#fff", fontSize: "1.3rem", letterSpacing: "0.06em", textTransform: "uppercase", margin: "0 0 0.6rem" }}>
          {t.title}
        </h2>
        <p style={{ fontFamily: '"BerlinType", sans-serif', color: "rgba(255,255,255,0.6)", fontSize: "0.9rem", lineHeight: 1.5, margin: "0 0 1.5rem" }}>
          {t.body}
        </p>
        <button
          type="button"
          onClick={close}
          style={{ background: busy ? "rgba(255,255,255,0.08)" : TEAL, border: busy ? "1px solid rgba(255,255,255,0.15)" : "none", borderRadius: 12, color: "#fff",
            fontFamily: '"Borsok", sans-serif', fontSize: "0.9rem", letterSpacing: "0.07em", textTransform: "uppercase", padding: "0.75rem 2.25rem", cursor: "pointer" }}
        >
          {busy ? "Закрыть" : "Хорошо"}
        </button>
      </div>
    </div>,
    document.body,
  );
}
