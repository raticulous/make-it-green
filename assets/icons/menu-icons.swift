// Renders the macOS tray menu icons into cmd/make-it-green-gui/icons/.
// Run from the repo root: swift assets/icons/menu-icons.swift
// Item icons are SF Symbols drawn black (shown as template images), the status dots keep their color.
import AppKit

let out = "cmd/make-it-green-gui/icons/", px = 32

func png(_ name: String, _ draw: (NSRect) -> Void) {
    let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: px, pixelsHigh: px, bitsPerSample: 8, samplesPerPixel: 4,
                               hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
    NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)
    draw(NSRect(x: 0, y: 0, width: px, height: px))
    NSGraphicsContext.current = nil
    try! rep.representation(using: .png, properties: [:])!.write(to: URL(fileURLWithPath: out + name + ".png"))
}

for (name, symbol) in ["menu-open": "macwindow", "menu-stop": "stop.circle", "menu-start": "play.circle", "menu-quit": "power"] {
    let img = NSImage(systemSymbolName: symbol, accessibilityDescription: nil)!
        .withSymbolConfiguration(.init(pointSize: 24, weight: .regular))!
    png(name) { box in
        let s = img.size, scale = min(26 / s.width, 26 / s.height)
        let w = s.width * scale, h = s.height * scale
        img.draw(in: NSRect(x: (box.width - w) / 2, y: (box.height - h) / 2, width: w, height: h))
    }
}

for (name, hex) in ["status-on": 0x34c759, "status-paused": 0xe8a91f, "status-off": 0x8e8e93] {
    png(name) { box in
        NSColor(red: CGFloat(hex >> 16) / 255, green: CGFloat(hex >> 8 & 0xff) / 255, blue: CGFloat(hex & 0xff) / 255, alpha: 1).setFill()
        NSBezierPath(ovalIn: box.insetBy(dx: 8, dy: 8)).fill()
    }
}
