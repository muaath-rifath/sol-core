export interface Board {
  id: string
  name: string
  family: string
}

const INDEX_URL = "https://raw.githubusercontent.com/espressif/arduino-esp32/gh-pages/package_esp32_index.json"
const BOARDS_TXT_URL = "https://raw.githubusercontent.com/espressif/arduino-esp32/master/boards.txt"

export async function fetchBoards(): Promise<Board[]> {
  try {
    const response = await fetch(BOARDS_TXT_URL)
    if (!response.ok) throw new Error("Failed to fetch boards.txt")
    const text = await response.text()
    return parseBoardsTxt(text)
  } catch (error) {
    console.error("Error fetching boards:", error)
    // Fallback to basic targets if fetching full list fails
    return [
      { id: "esp32", name: "Generic ESP32", family: "esp32" },
      { id: "esp32s2", name: "Generic ESP32-S2", family: "esp32s2" },
      { id: "esp32s3", name: "Generic ESP32-S3", family: "esp32s3" },
      { id: "esp32c3", name: "Generic ESP32-C3", family: "esp32c3" },
      { id: "esp32c6", name: "Generic ESP32-C6", family: "esp32c6" },
      { id: "esp32h2", name: "Generic ESP32-H2", family: "esp32h2" },
    ]
  }
}

function parseBoardsTxt(text: string): Board[] {
  const lines = text.split("\n")
  const boardsMap = new Map<string, Partial<Board>>()

  for (const line of lines) {
    const trimmed = line.trim()
    if (!trimmed || trimmed.startsWith("#")) continue

    const [key, value] = trimmed.split("=")
    if (!key || value === undefined) continue

    const parts = key.split(".")
    const boardId = parts[0]
    if (!boardId) continue

    if (!boardsMap.has(boardId)) {
      boardsMap.set(boardId, { id: boardId })
    }

    const board = boardsMap.get(boardId)!

    // Look for board name
    if (parts.length === 2 && parts[1] === "name") {
      board.name = value
    }

    // Look for build target (family)
    if (parts.length === 3 && parts[1] === "build" && parts[2] === "mcu") {
      board.family = value
    }
  }

  return Array.from(boardsMap.values())
    .filter((b): b is Board => !!(b.id && b.name && b.family))
    .sort((a, b) => a.name.localeCompare(b.name))
}
