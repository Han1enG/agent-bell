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
        bell.move(to: NSPoint(x: 20, y: 25))
        bell.curve(to: NSPoint(x: 29, y: 63), controlPoint1: NSPoint(x: 29, y: 36), controlPoint2: NSPoint(x: 27, y: 50))
        bell.curve(to: NSPoint(x: 44, y: 81), controlPoint1: NSPoint(x: 31, y: 73), controlPoint2: NSPoint(x: 36, y: 79))
        bell.curve(to: NSPoint(x: 56, y: 81), controlPoint1: NSPoint(x: 43, y: 94), controlPoint2: NSPoint(x: 57, y: 94))
        bell.curve(to: NSPoint(x: 71, y: 63), controlPoint1: NSPoint(x: 64, y: 79), controlPoint2: NSPoint(x: 69, y: 73))
        bell.curve(to: NSPoint(x: 80, y: 25), controlPoint1: NSPoint(x: 73, y: 50), controlPoint2: NSPoint(x: 71, y: 36))
        bell.curve(to: NSPoint(x: 74, y: 19), controlPoint1: NSPoint(x: 84, y: 20), controlPoint2: NSPoint(x: 80, y: 19))
        bell.line(to: NSPoint(x: 26, y: 19))
        bell.curve(to: NSPoint(x: 20, y: 25), controlPoint1: NSPoint(x: 20, y: 19), controlPoint2: NSPoint(x: 16, y: 20))
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
    static func menu(paused: Bool) -> NSImage {
        // Render eagerly: a status item's image must not depend on mutable model state.
        let image = NSImage(size: NSSize(width: 20, height: 20))
        image.lockFocus(); draw(in: NSRect(x: 0, y: 0, width: 20, height: 20), color: .black)
        if paused {
            NSColor.black.setStroke()
            let slash = NSBezierPath(); slash.lineWidth = 1.3
            slash.move(to: NSPoint(x: 2, y: 18)); slash.line(to: NSPoint(x: 18, y: 2)); slash.stroke()
        }
        image.unlockFocus(); image.isTemplate = true
        return image
    }
}
