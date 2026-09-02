export type FirmwareTemplateId = "switch"

export interface FlashConfig {
  wifiSsid: string
  wifiPassword: string
  mqttBrokerUri: string
  deviceId: string
  templateId: FirmwareTemplateId
  relayPins: [number, number, number, number]
  relayActiveLowMask: number
  mtls: {
    caCert: string
    clientCert: string
    clientKey: string
  }
}

export const PATCH_SIGNATURE = "SOLCFGv2::ESP32"
export const PATCH_SIGNATURE_V1 = "SOLCFGv1::ESP32"
export const PATCH_SIGNATURE_FIELD_SIZE = 16

export const SLOT_MAP = {
  wifiSsid: { fieldSize: 33, maxLength: 32, label: "Wi-Fi SSID" },
  wifiPassword: { fieldSize: 65, maxLength: 64, label: "Wi-Fi password" },
  mqttBrokerUri: { fieldSize: 129, maxLength: 128, label: "MQTT broker URI" },
  mqttUsername: { fieldSize: 65, maxLength: 64, label: "MQTT username" },
  mqttPassword: { fieldSize: 65, maxLength: 64, label: "MQTT password" },
  deviceId: { fieldSize: 65, maxLength: 64, label: "Device ID" },
  templateId: { fieldSize: 33, maxLength: 32, label: "Template ID" },
} as const

export const PATCH_BLOB_SIZE =
  PATCH_SIGNATURE_FIELD_SIZE +
  SLOT_MAP.wifiSsid.fieldSize +
  SLOT_MAP.wifiPassword.fieldSize +
  SLOT_MAP.mqttBrokerUri.fieldSize +
  SLOT_MAP.mqttUsername.fieldSize +
  SLOT_MAP.mqttPassword.fieldSize +
  SLOT_MAP.deviceId.fieldSize +
  SLOT_MAP.templateId.fieldSize +
  4 +
  1 +
  11

export const FIELD_OFFSETS = {
  wifiSsid: PATCH_SIGNATURE_FIELD_SIZE,
  wifiPassword: PATCH_SIGNATURE_FIELD_SIZE + SLOT_MAP.wifiSsid.fieldSize,
  mqttBrokerUri: PATCH_SIGNATURE_FIELD_SIZE + SLOT_MAP.wifiSsid.fieldSize + SLOT_MAP.wifiPassword.fieldSize,
  mqttUsername:
    PATCH_SIGNATURE_FIELD_SIZE + SLOT_MAP.wifiSsid.fieldSize + SLOT_MAP.wifiPassword.fieldSize + SLOT_MAP.mqttBrokerUri.fieldSize,
  mqttPassword:
    PATCH_SIGNATURE_FIELD_SIZE +
    SLOT_MAP.wifiSsid.fieldSize +
    SLOT_MAP.wifiPassword.fieldSize +
    SLOT_MAP.mqttBrokerUri.fieldSize +
    SLOT_MAP.mqttUsername.fieldSize,
  deviceId:
    PATCH_SIGNATURE_FIELD_SIZE +
    SLOT_MAP.wifiSsid.fieldSize +
    SLOT_MAP.wifiPassword.fieldSize +
    SLOT_MAP.mqttBrokerUri.fieldSize +
    SLOT_MAP.mqttUsername.fieldSize +
    SLOT_MAP.mqttPassword.fieldSize,
  templateId:
    PATCH_SIGNATURE_FIELD_SIZE +
    SLOT_MAP.wifiSsid.fieldSize +
    SLOT_MAP.wifiPassword.fieldSize +
    SLOT_MAP.mqttBrokerUri.fieldSize +
    SLOT_MAP.mqttUsername.fieldSize +
    SLOT_MAP.mqttPassword.fieldSize +
    SLOT_MAP.deviceId.fieldSize,
  relayPins:
    PATCH_SIGNATURE_FIELD_SIZE +
    SLOT_MAP.wifiSsid.fieldSize +
    SLOT_MAP.wifiPassword.fieldSize +
    SLOT_MAP.mqttBrokerUri.fieldSize +
    SLOT_MAP.mqttUsername.fieldSize +
    SLOT_MAP.mqttPassword.fieldSize +
    SLOT_MAP.deviceId.fieldSize +
    SLOT_MAP.templateId.fieldSize,
  relayActiveLowMask:
    PATCH_SIGNATURE_FIELD_SIZE +
    SLOT_MAP.wifiSsid.fieldSize +
    SLOT_MAP.wifiPassword.fieldSize +
    SLOT_MAP.mqttBrokerUri.fieldSize +
    SLOT_MAP.mqttUsername.fieldSize +
    SLOT_MAP.mqttPassword.fieldSize +
    SLOT_MAP.deviceId.fieldSize +
    SLOT_MAP.templateId.fieldSize +
    4,
} as const

export const CERT_OFFSETS = {
  caCert: 0x0000,
  clientCert: 0x1000,
  clientKey: 0x2000,
} as const

// I2S audio pins: MAX98357A speaker (4=BCLK, 5=LRC, 6=DOUT) and INMP441 mic (15=WS, 16=SCK, 17=SD).
// Driving these as GPIO outputs will damage connected audio hardware.
export const RESERVED_GPIO_PINS = new Set([4, 5, 6, 15, 16, 17])

// Matches partitions.csv: config, data, nvs, 0x620000, 0x6000
export const CONFIG_PARTITION_OFFSET = 0x620000
export const CONFIG_PARTITION_SIZE = 0x6000
export const MODEL_PARTITION_OFFSET = 0x630000

function readUInt32LE(bytes: Uint8Array, offset: number): number {
  return (
    bytes[offset] |
    (bytes[offset + 1] << 8) |
    (bytes[offset + 2] << 16) |
    (bytes[offset + 3] << 24)
  ) >>> 0
}

function alignUp(value: number, alignment: number): number {
  return (value + alignment - 1) & ~(alignment - 1)
}

function findSignatureOffset(bytes: Uint8Array, signature: string, expectPostZeroBytes = 12): number {
  const encoder = new TextEncoder()
  const signatureBytes = encoder.encode(signature)
  const maxStart = bytes.length - PATCH_SIGNATURE_FIELD_SIZE

  for (let i = 0; i <= maxStart; i++) {
    let matched = true
    for (let j = 0; j < signatureBytes.length; j++) {
      if (bytes[i + j] !== signatureBytes[j]) {
        matched = false
        break
      }
    }

    if (!matched || bytes[i + signatureBytes.length] !== 0) {
      continue
    }

    for (let j = 0; j < expectPostZeroBytes; j++) {
      if (bytes[i + PATCH_SIGNATURE_FIELD_SIZE + j] !== 0) {
        matched = false
        break
      }
    }

    if (matched) {
      return i
    }
  }

  return -1
}

export function recomputeEspImageChecksum(bytes: Uint8Array): void {
  const ESP_IMAGE_MAGIC = 0xe9
  const ESP_IMAGE_HEADER_SIZE = 24
  const ESP_IMAGE_SEGMENT_HEADER_SIZE = 8
  const ESP_CHECKSUM_MAGIC = 0xef

  if (bytes.length < ESP_IMAGE_HEADER_SIZE || bytes[0] !== ESP_IMAGE_MAGIC) {
    throw new Error("Patched firmware is not a valid ESP image (bad magic).")
  }

  const segmentCount = bytes[1]
  let pos = ESP_IMAGE_HEADER_SIZE
  let checksum = ESP_CHECKSUM_MAGIC

  for (let i = 0; i < segmentCount; i++) {
    if (pos + ESP_IMAGE_SEGMENT_HEADER_SIZE > bytes.length) {
      throw new Error("ESP image is truncated before segment header.")
    }

    const dataLength = readUInt32LE(bytes, pos + 4)
    const dataStart = pos + ESP_IMAGE_SEGMENT_HEADER_SIZE
    const dataEnd = dataStart + dataLength

    if (dataEnd > bytes.length) {
      throw new Error("ESP image is truncated inside segment data.")
    }

    for (let j = dataStart; j < dataEnd; j++) {
      checksum ^= bytes[j]
    }

    pos = dataEnd
  }

  const checksumOffset = alignUp(pos + 1, 16) - 1
  if (checksumOffset >= bytes.length) {
    throw new Error("ESP image is truncated before checksum byte.")
  }

  bytes[checksumOffset] = checksum
}

export function findConfigBlobOffset(bytes: Uint8Array): number {
  const v2 = findSignatureOffset(bytes, PATCH_SIGNATURE, 12)
  if (v2 >= 0) return v2
  const v1 = findSignatureOffset(bytes, PATCH_SIGNATURE_V1, 12)
  if (v1 >= 0) return v1
  return -1
}

export function findPartitionOffset(bytes: Uint8Array, label: string): number {
  const ESP_IMAGE_MAGIC = 0xe9
  const ESP_PARTITION_TABLE_OFFSET = 0x8000
  const PARTITION_SIZE = 32

  // The partition table is usually at 0x8000
  if (bytes.length < ESP_PARTITION_TABLE_OFFSET + PARTITION_SIZE) return -1

  for (let i = 0; i < 95; i++) {
    const offset = ESP_PARTITION_TABLE_OFFSET + i * PARTITION_SIZE
    if (offset + PARTITION_SIZE > bytes.length) break

    // Magic bytes for partition table entry (0xAA 0x50)
    if (bytes[offset] !== 0xaa || bytes[offset + 1] !== 0x50) {
      if (bytes[offset] === 0xff && bytes[offset + 1] === 0xff) break // End of table
      continue
    }

    const partitionLabel = new TextDecoder().decode(bytes.slice(offset + 8, offset + 24)).replace(/\0/g, "")
    if (partitionLabel === label) {
      return readUInt32LE(bytes, offset + 24)
    }
  }

  return -1
}

function writeStringField(
  bytes: Uint8Array,
  blobOffset: number,
  fieldOffset: number,
  fieldSize: number,
  label: string,
  maxLength: number,
  value: string,
  encoder: TextEncoder,
): void {
  const valueBytes = encoder.encode(value)
  if (valueBytes.length > maxLength) {
    throw new Error(`${label} exceeds ${maxLength} bytes (UTF-8).`)
  }
  const start = blobOffset + fieldOffset
  bytes.fill(0, start, start + fieldSize)
  bytes.set(valueBytes, start)
}

/**
 * Build the certs partition binary (64KB = 0x10000 bytes).
 * Layout matches certs.h: CA at 0x0000, client cert at 0x1000, key at 0x2000.
 * Each slot is 4KB (0x1000). Returns a 64KB Uint8Array filled with 0xFF.
 */
export function buildCertsPartition(caCert: string, clientCert: string, clientKey: string): Uint8Array {
  const PART_SIZE = 0x10000 // 64KB — matches partitions.csv
  const SLOT_SIZE = 0x1000  // 4KB per cert
  const encoder = new TextEncoder()
  const buf = new Uint8Array(PART_SIZE).fill(0xff)

  const writeCert = (slotOffset: number, pem: string) => {
    const bytes = encoder.encode(pem + "\0")
    if (bytes.length > SLOT_SIZE) throw new Error("Certificate/Key exceeds 4KB slot size")
    buf.fill(0x00, slotOffset, slotOffset + SLOT_SIZE)
    buf.set(bytes, slotOffset)
  }

  writeCert(CERT_OFFSETS.caCert, caCert)
  writeCert(CERT_OFFSETS.clientCert, clientCert)
  writeCert(CERT_OFFSETS.clientKey, clientKey)
  return buf
}

export async function patchFirmware(bytes: Uint8Array, config: FlashConfig): Promise<Uint8Array> {
  const encoder = new TextEncoder()
  const blobOffset = findConfigBlobOffset(bytes)
  if (blobOffset < 0) {
    throw new Error("Firmware is missing runtime config blob. Build firmware first.")
  }
  if (blobOffset + PATCH_BLOB_SIZE > bytes.length) {
    throw new Error("Firmware runtime config blob is smaller than expected for v2 template mode.")
  }

  const signatureBytes = encoder.encode(PATCH_SIGNATURE)
  bytes.fill(0, blobOffset, blobOffset + PATCH_SIGNATURE_FIELD_SIZE)
  bytes.set(signatureBytes, blobOffset)

  writeStringField(bytes, blobOffset, FIELD_OFFSETS.wifiSsid, SLOT_MAP.wifiSsid.fieldSize, SLOT_MAP.wifiSsid.label, SLOT_MAP.wifiSsid.maxLength, config.wifiSsid, encoder)
  writeStringField(bytes, blobOffset, FIELD_OFFSETS.wifiPassword, SLOT_MAP.wifiPassword.fieldSize, SLOT_MAP.wifiPassword.label, SLOT_MAP.wifiPassword.maxLength, config.wifiPassword, encoder)
  writeStringField(bytes, blobOffset, FIELD_OFFSETS.mqttBrokerUri, SLOT_MAP.mqttBrokerUri.fieldSize, SLOT_MAP.mqttBrokerUri.label, SLOT_MAP.mqttBrokerUri.maxLength, config.mqttBrokerUri, encoder)
  // Auth is always mTLS via the certs partition — these slots are zeroed and never read.
  bytes.fill(0, blobOffset + FIELD_OFFSETS.mqttUsername, blobOffset + FIELD_OFFSETS.mqttUsername + SLOT_MAP.mqttUsername.fieldSize)
  bytes.fill(0, blobOffset + FIELD_OFFSETS.mqttPassword, blobOffset + FIELD_OFFSETS.mqttPassword + SLOT_MAP.mqttPassword.fieldSize)
  writeStringField(bytes, blobOffset, FIELD_OFFSETS.deviceId, SLOT_MAP.deviceId.fieldSize, SLOT_MAP.deviceId.label, SLOT_MAP.deviceId.maxLength, config.deviceId, encoder)
  writeStringField(bytes, blobOffset, FIELD_OFFSETS.templateId, SLOT_MAP.templateId.fieldSize, SLOT_MAP.templateId.label, SLOT_MAP.templateId.maxLength, config.templateId, encoder)

  if (!Array.isArray(config.relayPins) || config.relayPins.length !== 4) {
    throw new Error("relayPins must include exactly 4 entries.")
  }
  for (let i = 0; i < 4; i++) {
    const pin = Number(config.relayPins[i])
    if (!Number.isInteger(pin) || pin < 0 || pin > 255) {
      throw new Error(`Relay pin at channel ${i + 1} must be an integer between 0 and 255.`)
    }
    if (RESERVED_GPIO_PINS.has(pin)) {
      throw new Error(`Relay channel ${i} GPIO ${pin} is reserved for I2S audio (MAX98357A/INMP441) — flashing this pin could damage hardware.`)
    }
    bytes[blobOffset + FIELD_OFFSETS.relayPins + i] = pin
  }

  const activeLowMask = Number(config.relayActiveLowMask)
  if (!Number.isInteger(activeLowMask) || activeLowMask < 0 || activeLowMask > 255) {
    throw new Error("Relay active-low mask must be an integer between 0 and 255.")
  }
  bytes[blobOffset + FIELD_OFFSETS.relayActiveLowMask] = activeLowMask
  bytes.fill(0, blobOffset + FIELD_OFFSETS.relayActiveLowMask + 1, blobOffset + PATCH_BLOB_SIZE)

  recomputeEspImageChecksum(bytes)

  if (bytes[23] === 1) {
    const hashBuf = await crypto.subtle.digest("SHA-256", bytes.slice(0, bytes.length - 32))
    bytes.set(new Uint8Array(hashBuf), bytes.length - 32)
  }

  return bytes
}
