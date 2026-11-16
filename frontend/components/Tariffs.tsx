"use client";
import BookingButton from "./BookingButton";
// ROMANOV RECORDS — Тарифы section
// LAYOUT: 2 × 2 compact cards (vocal, package / mixing+mastering, turnkey)
// HOVER: colour inversion via CSS classes tariff-light / tariff-dark (defined in globals.css)
//   tariff-light (cream bg / dark text)  →  hover becomes dark teal
//   tariff-dark  (dark bg / white text)  →  hover becomes cream
// Each card has: title, "записаться" btn, price

interface TariffCard {
  id: string;
  title: string;
  variant: "light" | "dark";
  price: React.ReactNode;
}

const DualPrice = () => (
  <div style={{ textAlign: "right", lineHeight: 1.35 }}>
    <div className="font-heading font-black" style={{ fontSize: "1.05rem" }}>
      2 500 ₽
    </div>
    <div className="font-body" style={{ fontSize: "0.7rem", opacity: 0.55 }}>
      со звукарем
    </div>
    <div className="font-heading font-black" style={{ fontSize: "0.95rem", marginTop: "0.2rem" }}>
      1 750 ₽
    </div>
    <div className="font-body" style={{ fontSize: "0.7rem", opacity: 0.55 }}>
      без звукаря
    </div>
  </div>
);

const TARIFFS: TariffCard[] = [
  {
    id: "vocals",
    title: "Запись вокала",
    variant: "light",
    price: <DualPrice />,
  },
  {
    id: "package",
    title: "Пакет запись + сведение",
    variant: "dark",
    price: <span className="tariff-price font-heading font-black text-xl">15 000 ₽</span>,
  },
  {
    id: "mixing",
    title: "Сведение + мастеринг",
    variant: "dark",
    price: <span className="tariff-price font-heading font-black text-xl">10 000 ₽</span>,
  },
  {
    id: "turnkey",
    title: "Трек под ключ",
    variant: "light",
    price: <span className="tariff-price font-heading font-black text-xl">от 35 000 ₽</span>,
  },
];

// Компактная плашка: название, кнопка записи и цена — без описаний
function TariffCard({ card }: { card: TariffCard }) {
  const base =
    "relative rounded-3xl p-7 flex flex-col justify-between gap-6 cursor-default " +
    (card.variant === "light" ? "tariff-light" : "tariff-dark");

  return (
    <div className={base}>
      <h3
        className="font-heading font-black text-xl leading-tight uppercase"
        style={{ letterSpacing: "0.02em" }}
      >
        {card.title}
      </h3>

      <div className="flex items-center justify-between gap-4 flex-wrap">
        <BookingButton className="btn-book text-sm" style={{ borderColor: "currentColor" }} />
        {card.price}
      </div>
    </div>
  );
}

export default function Tariffs() {

  return (
    <section id="tariffs" className="relative py-20 px-4 sm:px-8">
      {/* section title */}
      <div
        className="tariffs-section-title flex justify-center mb-12"
        style={{ display: "flex", paddingTop: "2rem" }}
      >
        <h2
          className="section-title font-heading font-black text-white uppercase"
          style={{ fontSize: "clamp(1.8rem, 4vw, 2.8rem)", letterSpacing: "0.06em" }}
        >
          Тарифы
        </h2>
      </div>

      <div className="max-w-5xl mx-auto grid grid-cols-1 md:grid-cols-2 gap-4">
        {TARIFFS.map((c) => <TariffCard key={c.id} card={c} />)}
      </div>
    </section>
  );
}
