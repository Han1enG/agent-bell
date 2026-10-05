import AppKit
let destination = CommandLine.arguments[1]
let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: 1024, pixelsHigh: 1024, bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
NSGraphicsContext.saveGraphicsState()
NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)
let tile = NSBezierPath(roundedRect: NSRect(x: 100, y: 100, width: 824, height: 824), xRadius: 185, yRadius: 185)
let gradient = NSGradient(starting: NSColor(calibratedRed: 0.10, green: 0.70, blue: 0.91, alpha: 1), ending: NSColor(calibratedRed: 0.035, green: 0.19, blue: 0.43, alpha: 1))!
gradient.draw(in: tile, angle: -70)
// Isolate the cutout so it reveals the tile beneath the white bell.
let mark = NSImage(size: NSSize(width: 620, height: 620))
mark.lockFocus(); BellIcon.draw(in: NSRect(x: 0, y: 0, width: 620, height: 620), color: .white); mark.unlockFocus()
mark.draw(in: NSRect(x: 202, y: 198, width: 620, height: 620))
NSGraphicsContext.restoreGraphicsState()
try rep.representation(using: .png, properties: [:])!.write(to: URL(fileURLWithPath: destination))
