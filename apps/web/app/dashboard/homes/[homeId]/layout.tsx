import DashboardSidebar from "@/components/DashboardSidebar"
import DashboardTopbar from "@/components/DashboardTopbar"
import { WSProvider } from "@/providers/WSProvider"

export default async function HomeLayout({
  children,
  params,
}: {
  children: React.ReactNode
  params: Promise<{ homeId: string }>
}) {
  const { homeId } = await params

  return (
    <WSProvider>
      <div className="bg-background text-on-background selection:bg-primary-container selection:text-on-primary-container min-h-screen antialiased">
        <DashboardTopbar homeId={homeId} />
        <div className="flex min-h-screen pt-24">
          <DashboardSidebar homeId={homeId} />
          <main className="relative flex flex-1 flex-col md:ml-[320px] px-4 pb-4">
            {children}
          </main>
        </div>
      </div>
    </WSProvider>
  )
}
