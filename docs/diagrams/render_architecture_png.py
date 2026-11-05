"""Render a lightweight PNG from the inline SVG source with Pillow."""

from pathlib import Path
import math
import re
import xml.etree.ElementTree as ET

from PIL import Image, ImageDraw, ImageFont


ROOT = Path(__file__).resolve().parent
html = (ROOT / "gcp-lab-architecture.html").read_text(encoding="utf-8")
css = re.search(r"<style>(.*?)</style>", html, re.S).group(1)
svg = re.search(r'(<svg xmlns="http://www.w3.org/2000/svg".*?</svg>)', html, re.S).group(1)
svg = svg.replace("<defs>", f"<style>{css}</style><defs>", 1)
(ROOT / "gcp-lab-architecture.svg").write_text(svg, encoding="utf-8")
svg_root = ET.fromstring(svg)
width, height = 1500, 820
scale, margin = 1.4, 24
image = Image.new("RGB", (int(width * scale + margin * 2), int(height * scale + margin * 2)), "#eceef2")
draw = ImageDraw.Draw(image)


def px(value: str | float) -> float:
    return float(value) * scale + margin


def class_styles(element: ET.Element) -> dict[str, str]:
    classes = element.attrib.get("class", "").split()
    style = {}
    # Theme colors are kept aligned with the branded HTML/SVG design source.
    mapping = {
        "node": {"fill": "#ffffff", "stroke": "#d7d9e0"},
        "cloud": {"fill": "#f1f6ff", "stroke": "#9dbbf2"},
        "gcp": {"fill": "#ffffff", "stroke": "#d7d9e0"},
        "focus": {"fill": "#fff3f4", "stroke": "#d10024"},
        "store": {"fill": "#f4f7f4", "stroke": "#95c69e"},
    }
    for name in classes:
        style.update(mapping.get(name, {}))
    return style


font_regular = r"C:\Windows\Fonts\arial.ttf"
font_bold = r"C:\Windows\Fonts\arialbd.ttf"

elements = list(svg_root.iter())
elements.sort(key=lambda item: 0 if item.tag.rsplit("}", 1)[-1] == "path" else 1)
for element in elements:
    tag = element.tag.rsplit("}", 1)[-1]
    if tag == "rect" and "x" in element.attrib:
        style = class_styles(element)
        box = (px(element.get("x", "0")), px(element.get("y", "0")), px(float(element.get("x", "0")) + float(element.get("width", "0"))), px(float(element.get("y", "0")) + float(element.get("height", "0"))))
        draw.rounded_rectangle(box, radius=px(14), fill=style.get("fill", "#ffffff"), outline=style.get("stroke", "#d7d9e0"), width=2)
    elif tag == "circle":
        cx, cy, radius = px(element.get("cx", "0")), px(element.get("cy", "0")), px(element.get("r", "0"))
        draw.ellipse((cx - radius, cy - radius, cx + radius, cy + radius), fill=element.get("fill", "#4285f4"))
    elif tag == "text":
        value = "".join(element.itertext()).strip()
        if not value:
            continue
        classes = element.get("class", "")
        size = 17 if "title" in classes else 12 if "sub" in classes else 11
        bold = "title" in classes or "gcpmark" in classes or "pill" in classes
        font = ImageFont.truetype(font_bold if bold else font_regular, int(size * scale))
        color = "#4285f4" if "gcpmark" in classes else "#ffffff" if "pill" in classes else "#15161d" if "title" in classes else "#5f6472"
        draw.text((px(element.get("x", "0")), px(element.get("y", "0")) - size * scale), value, font=font, fill=color)
    elif tag == "path" and element.get("d"):
        current = None
        segments = []
        for command, raw in re.findall(r"([A-Z])([^A-Z]*)", element.get("d", "")):
            args = [float(value) for value in re.findall(r"-?\d+(?:\.\d+)?", raw)]
            if command == "M" and len(args) >= 2:
                current = (args[0], args[1])
            elif command == "H" and current:
                target = (args[-1], current[1])
                segments.append((current, target))
                current = target
            elif command == "V" and current:
                target = (current[0], args[-1])
                segments.append((current, target))
                current = target
            elif command == "C" and current and len(args) >= 6:
                target = (args[-2], args[-1])
                segments.append((current, target))
                current = target
        for start, end in segments:
            start_px = (px(start[0]), px(start[1]))
            end_px = (px(end[0]), px(end[1]))
            dashed = "edge-dash" in element.get("class", "")
            if dashed:
                length = math.dist(start_px, end_px)
                if length:
                    ux = (end_px[0] - start_px[0]) / length
                    uy = (end_px[1] - start_px[1]) / length
                    for offset in range(0, int(length), 16):
                        draw.line((start_px[0] + ux * offset, start_px[1] + uy * offset, start_px[0] + ux * min(offset + 8, length), start_px[1] + uy * min(offset + 8, length)), fill="#8a91a0", width=2)
            else:
                draw.line((start_px, end_px), fill="#6d7380", width=3)
            angle = math.atan2(end_px[1] - start_px[1], end_px[0] - start_px[0])
            arrow = 10
            tip = end_px
            left = (tip[0] - arrow * math.cos(angle - 0.5), tip[1] - arrow * math.sin(angle - 0.5))
            right = (tip[0] - arrow * math.cos(angle + 0.5), tip[1] - arrow * math.sin(angle + 0.5))
            draw.polygon((tip, left, right), fill="#6d7380")

image.save(ROOT / "gcp-lab-architecture.png", optimize=True)
