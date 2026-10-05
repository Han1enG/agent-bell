import AppKit

// One vector mark for the app tile and the small monochrome menu-bar glyph.
enum BellIcon {
    static func draw(in rect: NSRect, color: NSColor) {
        let context = NSGraphicsContext.current!.cgContext
        context.saveGState()
        context.translateBy(x: rect.minX, y: rect.minY)
        context.scaleBy(x: rect.width / 100, y: rect.height / 100)
        color.setFill()
        let bell = NSBezierPath()
        bell.move(to: NSPoint(x: 15, y: 25))
        bell.curve(to: NSPoint(x: 24, y: 61), controlPoint1: NSPoint(x: 24, y: 36), controlPoint2: NSPoint(x: 22, y: 49))
        bell.curve(to: NSPoint(x: 43, y: 80), controlPoint1: NSPoint(x: 26, y: 72), controlPoint2: NSPoint(x: 34, y: 78))
        bell.curve(to: NSPoint(x: 57, y: 80), controlPoint1: NSPoint(x: 43, y: 93), controlPoint2: NSPoint(x: 57, y: 93))
        bell.curve(to: NSPoint(x: 76, y: 61), controlPoint1: NSPoint(x: 66, y: 78), controlPoint2: NSPoint(x: 74, y: 72))
        bell.curve(to: NSPoint(x: 85, y: 25), controlPoint1: NSPoint(x: 78, y: 49), controlPoint2: NSPoint(x: 76, y: 36))
        bell.curve(to: NSPoint(x: 78, y: 19), controlPoint1: NSPoint(x: 89, y: 20), controlPoint2: NSPoint(x: 85, y: 19))
        bell.line(to: NSPoint(x: 22, y: 19))
        bell.curve(to: NSPoint(x: 15, y: 25), controlPoint1: NSPoint(x: 15, y: 19), controlPoint2: NSPoint(x: 11, y: 20))
        bell.close(); bell.fill()
        let clapper = NSBezierPath()
        clapper.appendArc(withCenter: NSPoint(x: 50, y: 16), radius: 10, startAngle: 180, endAngle: 360)
        clapper.close(); clapper.fill()
        // Cut the terminal prompt out of the bell, keeping a true template image.
        context.setBlendMode(.destinationOut)
        NSColor.black.setStroke()
        let prompt = NSBezierPath(); prompt.lineWidth = 7; prompt.lineCapStyle = .round; prompt.lineJoinStyle = .round
        prompt.move(to: NSPoint(x: 38, y: 61)); prompt.line(to: NSPoint(x: 49, y: 50)); prompt.line(to: NSPoint(x: 38, y: 39)); prompt.stroke()
        let cursor = NSBezierPath(); cursor.lineWidth = 6; cursor.lineCapStyle = .round
        cursor.move(to: NSPoint(x: 55, y: 38)); cursor.line(to: NSPoint(x: 65, y: 38)); cursor.stroke()
        context.restoreGState()
    }
    static func menu(paused: Bool, count: Int, unread: Bool) -> NSImage {
        // Bell and badge share one fixed-size template image and one status item.
        let image = NSImage(size: NSSize(width: 26, height: 22))
        image.lockFocus()
        draw(in: NSRect(x: 0.5, y: 0, width: 22, height: 21), color: .black)
        if paused {
            NSColor.black.setStroke()
            let slash = NSBezierPath(); slash.lineWidth = 1.3
            slash.move(to: NSPoint(x: 2, y: 19)); slash.line(to: NSPoint(x: 21, y: 2)); slash.stroke()
        }
        let context = NSGraphicsContext.current!.cgContext
        if count > 0 {
            let text = count > 99 ? "99+" : String(count)
            let width: CGFloat = count > 99 ? 15 : (count > 9 ? 12 : 10)
            let badge = NSRect(x: 26 - width, y: 12, width: width, height: 10)
            context.setBlendMode(.destinationOut)
            NSColor.black.setFill()
            NSBezierPath(roundedRect: badge.insetBy(dx: -1.2, dy: -1.2), xRadius: 6, yRadius: 6).fill()
            context.setBlendMode(.normal)
            NSBezierPath(roundedRect: badge, xRadius: 5, yRadius: 5).fill()
            context.setBlendMode(.destinationOut)
            let attributes: [NSAttributedString.Key: Any] = [.font: NSFont.monospacedDigitSystemFont(ofSize: count > 9 ? 7 : 8, weight: .bold), .foregroundColor: NSColor.black]
            let label = NSAttributedString(string: text, attributes: attributes)
            let size = label.size()
            label.draw(at: NSPoint(x: badge.midX - size.width / 2, y: badge.midY - size.height / 2))
        } else if unread {
            context.setBlendMode(.destinationOut)
            NSColor.black.setFill(); NSBezierPath(ovalIn: NSRect(x: 18, y: 15, width: 6, height: 6)).fill()
            context.setBlendMode(.normal)
            NSBezierPath(ovalIn: NSRect(x: 19.25, y: 16.25, width: 3.5, height: 3.5)).fill()
        }
        context.setBlendMode(.normal)
        image.unlockFocus(); image.isTemplate = true
        return image
    }
}
