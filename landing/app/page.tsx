import Header from './components/Header'
import Hero from './components/Hero'
import Strip from './components/Strip'
import Cards from './components/Cards'
import Features from './components/Features'
import Screenshots from './components/Screenshots'
import GetSaturX from './components/GetSaturX'
import Footer from './components/Footer'

export default function Home() {
  return (
    <div className="min-h-screen w-full">
      <div className="h-1 bg-saturx-green" aria-hidden />
      <Header />
      <main>
        <Hero />
        <Strip />
        <Cards />
        <Features />
        <Screenshots />
        <GetSaturX />
        <Footer />
      </main>
    </div>
  )
}
