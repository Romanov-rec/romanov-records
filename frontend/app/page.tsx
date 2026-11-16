// ROMANOV RECORDS — main page
// Section order: Hero → Equipment → Tariffs → Location+Contacts
import Nav from "@/components/Nav";
import Hero from "@/components/Hero";
import Equipment from "@/components/Equipment";
import Tariffs from "@/components/Tariffs";
import Location from "@/components/Location";

export default function Home() {
  return (
    <>
      <Nav />
      <main>
        <Hero />
        <Equipment />
        <Tariffs />
        <Location />
      </main>
    </>
  );
}
