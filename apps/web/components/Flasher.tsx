"use client"

import { patchFirmware, buildCertsPartition, RESERVED_GPIO_PINS, type FlashConfig, type FirmwareTemplateId, CONFIG_PARTITION_OFFSET, CONFIG_PARTITION_SIZE, MODEL_PARTITION_OFFSET } from "@/lib/firmware-patcher"
import { getDeviceProvisioning } from "@/lib/actions"
import { IconAlertCircle, IconCheck, IconLoader2, IconX } from "@tabler/icons-react"
import clsx from "clsx"
import { useCallback, useEffect, useMemo, useRef, useState } from "react"

type FirmwareVersion = {
  id: string
  template_id: string
  version: string
}

type DeviceOption = {
  id: string
  name: string
  room_id: string
  metadata?: Record<string, string>
}

type LogEntry = {
  text: string
  type: "info" | "error" | "success"
}

type ApplianceOption = {
  device_id: string
  channel?: number
  gpio_pin?: number
  active_low: boolean
}

const DEFAULT_RELAY_PINS: [number, number, number, number] = [12, 13, 14, 21]

type Props = {
  firmwareVersions: FirmwareVersion[]
  devices: DeviceOption[]
  appliances?: ApplianceOption[]
  defaultTemplate?: string
  mqttBrokerUrl: string
  caCert: string
}

export default function Flasher({
  firmwareVersions,
  devices,
  appliances,
  defaultTemplate,
  mqttBrokerUrl,
  caCert,
}: Props) {
  const [deviceID, setDeviceID] = useState<string>(devices[0]?.id ?? "")
  const [selectedFirmwareId, setSelectedFirmwareId] = useState<string>(firmwareVersions[0]?.id ?? "")
  const [wifiSsid, setWifiSsid] = useState("")
  const [wifiPassword, setWifiPassword] = useState("")
  const [relayPins, setRelayPins] = useState<[number, number, number, number]>([...DEFAULT_RELAY_PINS] as [number, number, number, number])
  const [isBusy, setIsBusy] = useState(false)
  const [status, setStatus] = useState<string>("Ready")
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [showLogs, setShowLogs] = useState(false)

  const logEndRef = useRef<HTMLDivElement>(null)

  const selectedDevice = useMemo(
    () => devices.find((d) => d.id === deviceID) ?? devices[0],
    [deviceID, devices]
  )

  const selectedFirmware = useMemo(() => {
    return firmwareVersions.find((f) => f.id === selectedFirmwareId) ?? firmwareVersions[0]
  }, [selectedFirmwareId, firmwareVersions])

  useEffect(() => {
    if (!selectedFirmwareId && firmwareVersions.length > 0) {
      setSelectedFirmwareId(firmwareVersions[0].id)
    }
  }, [firmwareVersions, selectedFirmwareId])

  useEffect(() => {
    if (!appliances || !deviceID) {
      setRelayPins([...DEFAULT_RELAY_PINS] as [number, number, number, number])
      return
    }
    const pins: [number, number, number, number] = [...DEFAULT_RELAY_PINS] as [number, number, number, number]
    for (const app of appliances) {
      if (app.device_id === deviceID && app.channel != null && app.channel >= 0 && app.channel < 4 && app.gpio_pin != null) {
        pins[app.channel] = app.gpio_pin
      }
    }
    setRelayPins(pins)
  }, [deviceID, appliances])

  const relayActiveLowMask = useMemo(() => {
    if (!appliances || !deviceID) return 0
    let mask = 0
    for (const app of appliances) {
      if (app.device_id === deviceID && app.channel != null && app.channel >= 0 && app.channel < 4 && app.active_low) {
        mask |= (1 << app.channel)
      }
    }
    return mask
  }, [deviceID, appliances])

  function firmwareLabel(firmware: FirmwareVersion, index: number) {
    const tags = []
    if (index === 0) tags.push("latest")
    if (selectedDevice?.metadata?.firmware_id === firmware.id) tags.push("assigned")

    return tags.length
      ? `${firmware.version} (${firmware.template_id}, ${tags.join(", ")})`
      : `${firmware.version} (${firmware.template_id})`
  }

  const addLog = useCallback((text: string, type: "info" | "error" | "success" = "info") => {
    setLogs((prev) => [...prev, { text, type }])
  }, [])

  useEffect(() => {
    if (logEndRef.current) {
      logEndRef.current.scrollIntoView({ behavior: "smooth" })
    }
  }, [logs])

  function getFriendlyErrorMessage(message: string): string[] {
    if (
      message.includes("Failed to execute 'open' on 'SerialPort'") ||
      message.includes("Failed to open serial port")
    ) {
      return [
        "Failed to open serial port.",
        "Close anything using the port (idf.py monitor, screen, minicom, another browser tab), then retry.",
        "On Linux, ModemManager may grab CH340 ports: sudo systemctl stop ModemManager",
        "For a permanent fix: sudo systemctl disable ModemManager",
        "On Linux, ensure your user is in dialout: sudo usermod -aG dialout $USER (then log out/in).",
        "CH340 adapters usually appear as /dev/ttyUSB*.",
      ]
    }

    if (message.includes("No port selected") || message.includes("user gesture")) {
      return ["No serial port selected."]
    }

    if (
      message.includes("Failed to connect with the device") ||
      message.includes("timed out waiting for packet header")
    ) {
      return [
        "Failed to connect to ESP32 bootloader.",
        "Press and hold BOOT, tap RESET, release RESET, then release BOOT and retry immediately.",
        "If your board has CH340, auto-reset wiring may be missing; manual bootloader entry is often required.",
      ]
    }

    if (
      message.includes("The device has been lost") ||
      message.includes("device has been lost")
    ) {
      return [
        "USB serial device was lost during flashing.",
        "Close idf.py monitor or any serial tool using the same port, then retry.",
        "If monitor is open, press Ctrl+] in that terminal to exit it cleanly.",
        "If this keeps happening, unplug/replug the USB cable and try again.",
      ]
    }

    return [`Error: ${message}`]
  }

  async function onFlash() {
    if (!deviceID) {
      setStatus("Select a device first")
      return
    }
    if (!selectedFirmware) {
      setStatus("No firmware binary uploaded yet — go to Manage Firmware and upload one first")
      return
    }
    if (!wifiSsid.trim()) {
      setStatus("Please enter a Wi-Fi SSID")
      return
    }

    const nav = navigator as Navigator & {
      serial?: {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        requestPort: (options?: { filters?: { usbVendorId: number; usbProductId?: number }[] }) => Promise<any>
      }
    }
    if (!nav.serial) {
      setStatus("WebSerial is not supported in this browser")
      return
    }

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let port: any
    try {
      port = await nav.serial.requestPort({
        filters: [
          { usbVendorId: 0x1a86, usbProductId: 0x55d3 }, // CH340
          { usbVendorId: 0x303a }, // Espressif USB interfaces
        ],
      })
    } catch (err) {
      console.error("User cancelled or failed to select port", err)
      return
    }

    setIsBusy(true)
    setShowLogs(true)
    setLogs([])
    try {
      addLog("Starting flash process...")
      setStatus("Downloading firmware...")
      addLog(`Selected firmware: ${selectedFirmware.version} (${selectedFirmware.template_id}, ${selectedFirmware.id})`)
      addLog(`Downloading firmware version ${selectedFirmware.version}...`)
      const [response, blResponse, ptResponse, modelResponse] = await Promise.all([
        fetch(`/api/firmware/${selectedFirmware.id}`, { cache: "no-store" }),
        fetch(`/api/firmware/${selectedFirmware.id}/bootloader`, { cache: "no-store" }),
        fetch(`/api/firmware/${selectedFirmware.id}/partition-table`, { cache: "no-store" }),
        fetch(`/api/firmware/${selectedFirmware.id}/model`, { cache: "no-store" }),
      ])
      if (!response.ok) {
        throw new Error(`Download failed: ${response.status}`)
      }
      const arrayBuffer = await response.arrayBuffer()
      const bytes = new Uint8Array(arrayBuffer)
      let bootloaderBinary: Uint8Array | null = null
      if (blResponse.ok) {
        bootloaderBinary = new Uint8Array(await blResponse.arrayBuffer())
        addLog(`Bootloader downloaded (${bootloaderBinary.byteLength} bytes)`)
      } else {
        addLog("Warning: bootloader not available — device bootloader will not be updated", "error")
      }
      let partitionTable: Uint8Array | null = null
      if (ptResponse.ok) {
        partitionTable = new Uint8Array(await ptResponse.arrayBuffer())
        addLog(`Partition table downloaded (${partitionTable.byteLength} bytes)`)
      } else {
        addLog("Warning: partition table not available — certs partition may not be found on device", "error")
      }
      addLog("Downloading wake word model binary...")
      let modelBinary: Uint8Array | null = null
      if (modelResponse.ok) {
        modelBinary = new Uint8Array(await modelResponse.arrayBuffer())
        addLog(`Model binary downloaded (${modelBinary.byteLength} bytes)`)
      } else {
        addLog("No model binary for this firmware version — skipping model partition")
      }

      addLog("Generating device mTLS certificates...")
      const certBundle = await getDeviceProvisioning(deviceID).catch((e) => {
        throw new Error(`Certificate generation failed: ${e instanceof Error ? e.message : String(e)}`)
      })
      if (!certBundle) {
        throw new Error("mTLS provisioning is disabled on the server — cannot flash without certificates. Enable it in sol-core configuration.")
      }
      addLog("Certificates generated successfully.")

      setStatus("Patching firmware...")
      addLog("Patching SOLCFGv2 fields...")
      const patched = await patchFirmware(bytes, {
        wifiSsid,
        wifiPassword,
        mqttBrokerUri: mqttBrokerUrl,
        deviceId: deviceID,
        templateId: (selectedFirmware.template_id || defaultTemplate || "switch") as FirmwareTemplateId,
        relayPins: relayPins,
        relayActiveLowMask: relayActiveLowMask,
        mtls: {
          caCert,
          clientCert: certBundle.CertificatePEM,
          clientKey: certBundle.PrivateKeyPEM,
        },
      })

      // Build certs partition binary (64KB at 0x610000)
      let certsPartition: Uint8Array | null = null
      if (certBundle) {
        addLog("Building certs partition binary...")
        certsPartition = buildCertsPartition(
          caCert,
          certBundle.CertificatePEM,
          certBundle.PrivateKeyPEM,
        )
      }

      setStatus("Flashing with esptool-js...")
      addLog("Connecting to ESP32...")
      const mod = (await import("esptool-js")) as unknown as {
        ESPLoader?: new (...args: unknown[]) => {
          main: () => Promise<void>
          writeFlash: (...args: unknown[]) => Promise<void>
          after: (mode: string) => Promise<void>
        }
        Transport?: new (...args: unknown[]) => unknown
      }

      if (!mod.ESPLoader || !mod.Transport) {
        throw new Error("esptool-js exports not found")
      }

      const transport = new mod.Transport(port)
      const loader = new mod.ESPLoader({
        transport,
        baudrate: 921600,
        terminal: {
          clean: () => setLogs([]),
          writeLine: (data: string) => addLog(data),
          write: (data: string) => addLog(data),
        },
      })

      await loader.main()
      addLog("Chip connected. Writing flash...")
      await loader.writeFlash({
        fileArray: [
          // Bootloader at 0x0000 — must match the IDF version the app was built with
          ...(bootloaderBinary ? [{ address: 0x0000, data: bootloaderBinary }] : []),
          // Partition table at 0x8000 — required for firmware to locate the certs partition
          ...(partitionTable ? [{ address: 0x8000, data: partitionTable }] : []),
          {
            address: 0x10000,
            data: patched,
          },
          // Write the certs partition to 0x610000 (matches partitions.csv)
          ...(certsPartition ? [{ address: 0x610000, data: certsPartition }] : []),
          // Write WakeNet model partition at 0x630000 (matches partitions.csv "model" entry)
          ...(modelBinary ? [{ address: MODEL_PARTITION_OFFSET, data: modelBinary }] : []),
          // Reset OTA data to force boot from ota_0
          {
            address: 0xe000,
            data: new Uint8Array(8192).fill(0xff),
          },
          // Erase the config NVS partition so the freshly-patched blob is
          // picked up on first boot instead of stale NVS values.
          {
            address: CONFIG_PARTITION_OFFSET,
            data: new Uint8Array(CONFIG_PARTITION_SIZE).fill(0xff),
          },
        ],
        flashSize: "keep",
        flashMode: "keep",
        flashFreq: "keep",
        eraseAll: false,
        compress: true,
      })
      addLog("Flash complete! Resetting...", "success")
      await loader.after("hard_reset")
      setStatus("Flash complete")
    } catch (error) {
      const msg = error instanceof Error ? error.message : String(error)
      for (const line of getFriendlyErrorMessage(msg)) {
        addLog(line, "error")
      }
      setStatus("Flash failed")
    } finally {
      setIsBusy(false)
    }
  }

  return (
    <div className="rounded-3xl border border-white/60 bg-surface-container-low p-5 shadow-[10px_10px_24px_rgba(87,66,62,0.12),-10px_-10px_24px_rgba(255,255,255,0.92)]">
      <h2 className="font-display text-lg font-semibold text-on-surface">WebSerial Flasher</h2>
      <p className="mt-1 text-sm text-on-surface-variant">Download firmware, patch SOLCFGv2 fields, and flash via serial.</p>

      <div className="mt-4 grid gap-3 md:grid-cols-2">
        <label className="text-xs font-semibold text-on-surface-variant">
          Node (Switchboard)
          <select
            value={deviceID}
            onChange={(e) => setDeviceID(e.target.value)}
            className="clay-inset mt-1 w-full rounded-xl border border-white/55 px-3 py-2 text-sm text-on-surface"
          >
            {devices.map((d) => (
              <option key={d.id} value={d.id}>
                {d.name}
              </option>
            ))}
          </select>
        </label>

        <label className="text-xs font-semibold text-on-surface-variant">
          Firmware Version
          <select
            value={selectedFirmwareId}
            onChange={(e) => setSelectedFirmwareId(e.target.value)}
            className="clay-inset mt-1 w-full rounded-xl border border-white/55 px-3 py-2 text-sm text-on-surface"
          >
            {firmwareVersions.map((f, index) => (
              <option key={f.id} value={f.id}>
                {firmwareLabel(f, index)}
              </option>
            ))}
          </select>
        </label>

        <label className="col-span-full text-xs font-semibold text-on-surface-variant md:col-span-1">
          Wi-Fi SSID
          <input value={wifiSsid} onChange={(e) => setWifiSsid(e.target.value)} className="clay-inset mt-1 w-full rounded-xl border border-white/55 px-3 py-2 text-sm text-on-surface" />
        </label>

        <label className="col-span-full text-xs font-semibold text-on-surface-variant md:col-span-1">
          Wi-Fi Password
          <input type="password" value={wifiPassword} onChange={(e) => setWifiPassword(e.target.value)} className="clay-inset mt-1 w-full rounded-xl border border-white/55 px-3 py-2 text-sm text-on-surface" />
        </label>

        <div className="col-span-full space-y-1.5">
          <p className="text-xs font-semibold text-on-surface-variant">Relay Channel GPIO Pins</p>
          <div className="grid grid-cols-4 gap-2">
            {([0, 1, 2, 3] as const).map((ch) => {
              const pin = relayPins[ch]
              const isReserved = RESERVED_GPIO_PINS.has(pin)
              return (
                <div key={ch} className="space-y-0.5">
                  <p className="text-[10px] font-bold uppercase tracking-wider text-outline">Ch {ch}</p>
                  <input
                    type="number"
                    value={pin}
                    min={0}
                    max={48}
                    onChange={(e) => {
                      const v = parseInt(e.target.value, 10)
                      if (!isNaN(v)) {
                        setRelayPins((prev) => {
                          const next = [...prev] as [number, number, number, number]
                          next[ch] = v
                          return next
                        })
                      }
                    }}
                    className={`clay-inset w-full rounded-xl border px-2 py-1.5 text-sm text-on-surface ${
                      isReserved ? "border-error bg-error-container/10" : "border-white/55"
                    }`}
                  />
                  {isReserved && (
                    <p className="text-[9px] font-semibold text-error">I2S audio pin!</p>
                  )}
                </div>
              )
            })}
          </div>
          {relayActiveLowMask !== 0 && (
            <p className="text-[10px] text-on-surface-variant">
              Active-low mask: <span className="font-mono text-primary">0x{relayActiveLowMask.toString(16).padStart(2, "0")}</span>
              {" "}(from appliance config)
            </p>
          )}
        </div>

        <div className="flex items-end">
          <p className="text-xs text-on-surface-variant">
            MQTT broker: <span className="font-mono text-primary">{mqttBrokerUrl}</span>
          </p>
        </div>
      </div>

      <div className="mt-4 flex flex-wrap items-center gap-3">
        {!firmwareVersions.length && (
          <p className="w-full rounded-xl bg-error-container px-3 py-2 text-xs text-on-error-container">
            No firmware binary found. Upload one on the{" "}
            <a href="../firmware" className="underline">Manage Firmware</a> page first.
          </p>
        )}
        <button
          type="button"
          onClick={onFlash}
          disabled={isBusy || !devices.length || !firmwareVersions.length}
          className="btn-primary px-4 py-2 text-sm font-semibold disabled:opacity-60"
        >
          {isBusy ? "Flashing..." : "Start Flash"}
        </button>
        <span className="text-sm text-on-surface-variant">{status}</span>
        {logs.length > 0 && (
          <button
            onClick={() => setShowLogs(true)}
            className="text-xs font-bold text-primary hover:underline"
          >
            View Logs
          </button>
        )}
      </div>

      {showLogs && (
        <div className="fixed inset-0 z-[100] flex items-center justify-center bg-stone-900/40 p-4 backdrop-blur-sm">
          <div className="bg-clay-canvas w-full max-w-2xl overflow-hidden rounded-[2.5rem] border border-white/60 shadow-2xl">
            <div className="flex items-center justify-between border-b border-white/40 p-6">
              <div className="flex items-center gap-3">
                <h2 className="font-display text-xl font-bold text-on-surface">Flashing Logs</h2>
                {isBusy && <IconLoader2 className="animate-spin text-primary" size={20} />}
                {status === "Flash complete" && <IconCheck className="text-emerald-500" size={20} />}
                {status === "Flash failed" && <IconAlertCircle className="text-error" size={20} />}
              </div>
              <button onClick={() => setShowLogs(false)} className="text-on-surface-variant hover:text-on-surface">
                <IconX size={24} />
              </button>
            </div>
            <div className="p-6">
              <div className="relative h-96 overflow-hidden rounded-2xl border border-white/55 bg-stone-950 font-mono text-xs text-stone-100">
                <div className="absolute inset-0 overflow-y-auto p-4">
                  {logs.map((log, i) => (
                    <div key={i} className={clsx("mb-1", {
                      "text-error": log.type === "error",
                      "text-emerald-400": log.type === "success",
                      "text-stone-300": log.type === "info"
                    })}>
                      {log.text}
                    </div>
                  ))}
                  <div ref={logEndRef} />
                </div>
              </div>
              <div className="mt-4 flex items-center justify-between">
                <span className="text-sm font-medium text-on-surface-variant">
                  Status: <span className={clsx("font-bold", {
                    "text-error": status === "Flash failed",
                    "text-emerald-600": status === "Flash complete",
                    "text-primary": isBusy
                  })}>{status}</span>
                </span>
                <button
                  onClick={() => setShowLogs(false)}
                  className="btn-outline px-6 py-2 text-sm font-semibold"
                >
                  Close
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
