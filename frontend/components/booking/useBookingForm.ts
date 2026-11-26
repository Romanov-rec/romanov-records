"use client";

import { useCallback, useEffect, useState } from "react";
import { createBooking, fetchBusyTimes } from "./api";
import {
  createEmptyServices,
  estimateTotal,
  hoursOf,
  localDateString,
  SERVICES,
  type ServiceID,
  type ServiceSelection,
} from "./config";

export function useBookingForm(isOpen: boolean, onClose: () => void) {
  const [acknowledgements, setAcknowledgements] = useState({ privacy: false, oferta: false, payment: false });
  const [consentError, setConsentError] = useState(false);
  const [name, setName] = useState("");
  const [phone, setPhone] = useState("");
  const [telegram, setTelegram] = useState("");
  const [selectedDate, setSelectedDate] = useState<Date | null>(null);
  const [selectedTime, setSelectedTime] = useState("");
  const [services, setServices] = useState<ServiceSelection>(createEmptyServices);
  const [comment, setComment] = useState("");
  const [calendarOpen, setCalendarOpen] = useState(false);
  const [timeOpen, setTimeOpen] = useState(false);
  const [busyTimes, setBusyTimes] = useState<string[]>([]);
  const [availabilityError, setAvailabilityError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState("");
  const [success, setSuccess] = useState(false);

  const chosenServiceIDs = SERVICES.filter((service) => services[service.id].checked)
    .map((service) => service.id);
  const onlyMixingSelected = chosenServiceIDs.length === 1 && chosenServiceIDs[0] === "mixing";
  // Запись по часам оплачивается онлайн сразу — сумма для кнопки и подсказки
  const payTotal = onlyMixingSelected ? 0 : estimateTotal(services);
  const payable = payTotal > 0;

  const reset = useCallback(() => {
    setAcknowledgements({ privacy: false, oferta: false, payment: false });
    setConsentError(false);
    setName("");
    setPhone("");
    setTelegram("");
    setSelectedDate(null);
    setSelectedTime("");
    setServices(createEmptyServices());
    setComment("");
    setCalendarOpen(false);
    setTimeOpen(false);
    setBusyTimes([]);
    setAvailabilityError("");
    setSubmitError("");
    setSuccess(false);
  }, []);

  const close = useCallback(() => {
    onClose();
    reset();
  }, [onClose, reset]);

  useEffect(() => {
    if (!isOpen) return;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") close();
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [isOpen, close]);

  useEffect(() => {
    document.body.style.overflow = isOpen ? "hidden" : "";
    return () => { document.body.style.overflow = ""; };
  }, [isOpen]);

  const selectDate = async (date: Date) => {
    setSelectedDate(date);
    setSelectedTime("");
    setCalendarOpen(false);
    setBusyTimes([]);
    setAvailabilityError("");
    try {
      setBusyTimes(await fetchBusyTimes(localDateString(date)));
    } catch {
      setAvailabilityError("Не удалось загрузить свободное время. Попробуйте выбрать дату ещё раз.");
    }
  };

  const toggleService = (id: ServiceID) => {
    setServices((current) => ({
      ...current,
      [id]: { ...current[id], checked: !current[id].checked },
    }));
  };

  const setServiceHours = (id: ServiceID, hours: string) => {
    setServices((current) => ({ ...current, [id]: { ...current[id], hours } }));
  };

  const setVocalEngineer = (engineer: boolean) => {
    setServices((current) => ({ ...current, vocal: { ...current.vocal, engineer } }));
  };

  const submit = async () => {
    if (submitting) return;
    setSubmitError("");
    if (!Object.values(acknowledgements).every(Boolean)) {
      setConsentError(true);
      const firstMissing = Object.entries(acknowledgements).find(([, checked]) => !checked)?.[0];
      document.getElementById(`booking-consent-${firstMissing}`)?.focus();
      return;
    }
    setConsentError(false);
    if (name.trim().length < 3) return setSubmitError("Введите имя (не менее 3 символов)");
    if (!phone.startsWith("+") || phone.replace(/\D/g, "").length < 10) {
      return setSubmitError("Номер телефона должен начинаться с + и содержать не менее 10 цифр");
    }
    if (telegram.trim() && !telegram.trim().startsWith("@")) {
      return setSubmitError("Telegram username должен начинаться с @");
    }

    const chosenServices = SERVICES.filter((service) => services[service.id].checked);
    if (chosenServices.length === 0) return setSubmitError("Выберите хотя бы одну услугу");

    const onlyMixing = chosenServices.length === 1 && chosenServices[0].id === "mixing";
    if (!onlyMixing && !selectedDate) return setSubmitError("Выберите дату записи");
    if (!onlyMixing && !selectedTime) return setSubmitError("Выберите время записи");

    const labelOf = (id: ServiceID, label: string) => id === "vocal"
      ? `${label} (${services.vocal.engineer ? "со звукорежиссёром" : "без звукорежиссёра"})`
      : label;
    const requestDetails = chosenServices
      .map((service) => service.hasHours
        ? `${labelOf(service.id, service.label)} — ${hoursOf(services[service.id].hours)} ч.`
        : service.label)
      .join(", ");
    const durationHours = Math.max(
      1,
      ...chosenServices
        .filter((service) => service.hasHours)
        .map((service) => hoursOf(services[service.id].hours)),
    );
    const chosenItems = chosenServices.map((service) => ({
      id: service.id,
      ...(service.hasHours ? { hours: hoursOf(services[service.id].hours) } : {}),
      ...(service.id === "vocal" ? { engineer: services.vocal.engineer } : {}),
    }));

    setSubmitting(true);
    let redirecting = false;
    try {
      const result = await createBooking({
        full_name: name.trim(),
        phone_number: phone,
        telegram_username: telegram,
        desired_date: onlyMixing ? "1970-01-01" : localDateString(selectedDate!),
        desired_time: onlyMixing ? "00:00" : selectedTime,
        duration_hours: durationHours,
        request_details: requestDetails,
        comment,
        services: chosenItems,
      });
      if (result.pay_url) {
        // Время закреплено за клиентом — отправляем на страницу оплаты ЮKassa
        redirecting = true;
        window.location.assign(result.pay_url);
        return;
      }
      setSuccess(true);
    } catch (error) {
      setSubmitError(error instanceof Error
        ? error.message
        : "Не удалось подключиться к серверу. Проверьте соединение.");
    } finally {
      if (!redirecting) setSubmitting(false);
    }
  };

  return {
    acknowledgements,
    setAcknowledgements,
    consentError,
    availabilityError,
    busyTimes,
    calendarOpen,
    close,
    comment,
    name,
    onlyMixingSelected,
    payable,
    payTotal,
    phone,
    selectedDate,
    selectedTime,
    selectDate,
    services,
    setCalendarOpen,
    setComment,
    setName,
    setPhone,
    setSelectedTime,
    setServiceHours,
    setTelegram,
    setTimeOpen,
    setVocalEngineer,
    submit,
    submitError,
    submitting,
    success,
    telegram,
    timeOpen,
    toggleService,
  };
}
