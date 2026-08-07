#!/usr/bin/env python3
"""Gera o papel de parede da conversa: rabiscos proprios ladrilhados sem costura.

Uso:  python3 tools/wallpaper/make_wallpaper.py
Saida: internal/web/static/img/wallpaper-{light,dark}.svg
"""
import math
import os
import random
import re

TILE = 680
STROKE = 1.25
SHAPES = ("path", "circle", "rect", "ellipse", "polygon", "polyline", "line")
DOODLES = {}


def non_scaling(markup):
    """Poe vector-effect em cada forma. Nao vale no <g>: vector-effect nao e
    propriedade herdada, entao no grupo nao surte efeito nenhum e a linha volta
    a escalar junto com o simbolo."""
    for tag in SHAPES:
        markup = re.sub(r"<" + tag + r"\b", f'<{tag} vector-effect="non-scaling-stroke"', markup)
    return markup


def sym(name, w, h, body):
    DOODLES[name] = (w, h, body.strip())


sym("saturn", 24, 20, """
<circle cx="12" cy="10" r="5.4"/>
<ellipse cx="12" cy="10" rx="11" ry="3.4" transform="rotate(-22 12 10)"/>
""")
sym("planet", 19, 19, """
<circle cx="9.5" cy="9.5" r="7.6"/>
<path d="M3 5.6c3.4 1.6 8.2 1.8 12 .4"/>
<path d="M2.4 12c3.8 1.8 9.4 1.6 13.6-.6"/>
""")
sym("star", 20, 19, '<path d="M10 1.6 12.6 7l6 .9-4.3 4.2 1 6-5.3-2.8-5.3 2.8 1-6L1.4 7.9l6-.9z"/>')
sym("sparkle", 16, 16, """
<path d="M8 1.2c.6 4 2.8 6.2 6.8 6.8-4 .6-6.2 2.8-6.8 6.8-.6-4-2.8-6.2-6.8-6.8 4-.6 6.2-2.8 6.8-6.8z"/>
""")
sym("heart", 20, 18, """
<path d="M10 16.4C3.4 12 1.4 8.8 1.4 6.2 1.4 3.6 3.4 2 5.6 2c1.8 0 3.4 1 4.4 2.6C11 3 12.6 2 14.4 2c2.2 0 4.2 1.6 4.2 4.2 0 2.6-2 5.8-8.6 10.2z"/>
""")
sym("rocket", 18, 25, """
<path d="M9 1.4c3.4 3 5.2 7.2 5.2 11.6L14 18H4l-.2-5C3.8 8.6 5.6 4.4 9 1.4z"/>
<circle cx="9" cy="9.4" r="2.2"/>
<path d="M4 13.4 1.4 17.6l3-.6M14 13.4l2.6 4.2-3-.6"/>
<path d="M7 18.6c.6 2.6 1.4 4.2 2 5 .6-.8 1.4-2.4 2-5"/>
""")
sym("coffee", 22, 20, """
<path d="M2.6 6.4h13.2v6.8c0 2.8-2.2 5-5 5H7.6c-2.8 0-5-2.2-5-5z"/>
<path d="M15.8 8.4h1.8a2.8 2.8 0 0 1 0 5.6h-1.8"/>
<path d="M6 3.6c-.8-1-.8-1.8 0-2.6M9.6 3.6c-.8-1-.8-1.8 0-2.6M13.2 3.6c-.8-1-.8-1.8 0-2.6"/>
""")
sym("camera", 24, 19, """
<rect x="1.6" y="4.6" width="20.8" height="13" rx="2.6"/>
<circle cx="12" cy="11" r="4.2"/>
<path d="M7.4 4.6 9 1.6h6l1.6 3"/>
<circle cx="18.6" cy="7.8" r="1"/>
""")
sym("note", 17, 21, """
<circle cx="4" cy="16.6" r="3"/><circle cx="13.4" cy="14.2" r="3"/>
<path d="M7 16.6V3.4l9.4-2.2v13"/><path d="M7 7.4l9.4-2.2"/>
""")
sym("smiley", 21, 21, """
<circle cx="10.5" cy="10.5" r="8.8"/>
<circle cx="7.6" cy="8.4" r="1.2"/><circle cx="13.4" cy="8.4" r="1.2"/>
<path d="M6.2 12.6c2.4 3.2 6.2 3.2 8.6 0"/>
""")
sym("wink", 21, 21, """
<circle cx="10.5" cy="10.5" r="8.8"/>
<path d="M6 8.4c1-1.2 2.4-1.2 3.4 0"/><circle cx="13.6" cy="8.4" r="1.2"/>
<path d="M6.4 12.8c2.4 3 6 3 8.4 0"/>
""")
sym("bolt", 14, 22, '<path d="M8.6 1.4 2 12.6h4.2L5.4 20.6 12 9.4H7.8z"/>')
sym("cloud", 25, 16, '<path d="M6.4 14.4a4.8 4.8 0 0 1-.6-9.6 6.4 6.4 0 0 1 12.2 1.4 4.1 4.1 0 0 1-.6 8.2z"/>')
sym("moon", 18, 19, '<path d="M13.6 1.8a8.6 8.6 0 1 0 2.8 13.8A9.4 9.4 0 0 1 13.6 1.8z"/>')
sym("terminal", 25, 19, """
<rect x="1.6" y="2.6" width="21.8" height="14.4" rx="2.2"/>
<path d="M1.6 6.4h21.8"/><path d="m6 10 2.4 2.2L6 14.4M11 14.4h5.4"/>
""")
sym("gear", 22, 22, """
<circle cx="11" cy="11" r="3.6"/>
<path d="M11 1.6v3M11 17.4v3M20.4 11h-3M4.6 11h-3M17.6 4.4l-2.1 2.1M6.5 15.5l-2.1 2.1M17.6 17.6l-2.1-2.1M6.5 6.5 4.4 4.4"/>
""")
sym("bug", 21, 21, """
<ellipse cx="10.5" cy="12" rx="5" ry="6.4"/><path d="M10.5 6.6V19"/>
<circle cx="10.5" cy="5.4" r="2.4"/><path d="M8.8 3.6 7.4 1.6M12.2 3.6l1.4-2"/>
<path d="M5.5 9 1.8 7.4M5.5 12.6H1.6M5.5 16.2l-3.4 2M15.5 9l3.7-1.6M15.5 12.6h3.9M15.5 16.2l3.4 2"/>
""")
sym("plane", 23, 20, '<path d="M21.4 1.6 1.6 9.4l7.4 2.8 2.6 6.8z"/><path d="M21.4 1.6 9 12.2"/>')
sym("diamond", 19, 19, '<path d="M9.5 1.6 17.4 9.5 9.5 17.4 1.6 9.5z"/>')
sym("flask", 19, 22, """
<path d="M7.4 1.8v6.6L2 18.2c-.8 1.6.3 3.4 2.1 3.4h10.8c1.8 0 2.9-1.8 2.1-3.4L11.6 8.4V1.8z"/>
<path d="M5.6 1.8h7.8"/><path d="M4.6 14.4h9.8"/>
""")
sym("chip", 21, 21, """
<rect x="5.4" y="5.4" width="10.2" height="10.2" rx="1.6"/>
<path d="M8.6 1.6v3.8M13.4 1.6v3.8M8.6 15.6v3.8M13.4 15.6v3.8M1.6 8.6h3.8M1.6 13.4h3.8M15.6 8.6h3.8M15.6 13.4h3.8"/>
""")
sym("folder", 23, 18, '<path d="M1.6 4.4a2 2 0 0 1 2-2h5l2.4 2.6h8.4a2 2 0 0 1 2 2v7.4a2 2 0 0 1-2 2H3.6a2 2 0 0 1-2-2z"/>')
sym("branch", 18, 22, """
<circle cx="4.4" cy="4.4" r="2.6"/><circle cx="4.4" cy="17.6" r="2.6"/>
<circle cx="13.6" cy="4.4" r="2.6"/><path d="M4.4 7v8"/>
<path d="M13.6 7v2.4c0 2.6-2 4.2-5 4.6"/>
""")
sym("ring", 16, 16, '<circle cx="8" cy="8" r="6.4"/>')
sym("dot", 9, 9, '<circle cx="4.5" cy="4.5" r="3.2"/>')

sym("donut", 22, 22, """
<circle cx="11" cy="11" r="9.4"/><circle cx="11" cy="11" r="3.4"/>
<path d="M4.6 5.4c1.6 1.2 1.2 3-.4 3.8M17 5.6c-1.4 1.4-.8 3 .8 3.6M8 19.4c.6-1.8 2.4-2 3.6-.8M17.8 15c-1.8.2-2.6 1.6-2 3.2"/>
""")
sym("pizza", 22, 21, """
<path d="M11 1.4 20.4 18a1.6 1.6 0 0 1-1.6 2.4H3.2A1.6 1.6 0 0 1 1.6 18z"/>
<path d="M4.6 17.6h12.8"/>
<circle cx="11" cy="8.6" r="1.4"/><circle cx="7.6" cy="13.4" r="1.2"/><circle cx="14.2" cy="13" r="1.2"/>
""")
sym("bicycle", 30, 19, """
<circle cx="6.4" cy="12.4" r="5.8"/><circle cx="23.6" cy="12.4" r="5.8"/>
<path d="M6.4 12.4 12 4.6h6l4.4 7.8M12 4.6h4.4M9.6 12.4h8.8"/>
<circle cx="6.4" cy="12.4" r="1"/>
""")
sym("guitar", 17, 26, """
<path d="M8.6 9.6c3 0 5.4 2.4 5.4 5.6 0 3.6-2.4 6.4-5.4 6.4-3 0-5.4-2.8-5.4-6.4 0-3.2 2.4-5.6 5.4-5.6z"/>
<circle cx="8.6" cy="14.6" r="2"/>
<path d="M8.6 9.6V3.2M6.8 1.6h3.6"/>
""")
sym("headphones", 24, 20, """
<path d="M2.6 13.6v-2a9.4 9.4 0 0 1 18.8 0v2"/>
<rect x="1" y="12.4" width="5" height="7" rx="2.4"/><rect x="18" y="12.4" width="5" height="7" rx="2.4"/>
""")
sym("icecream", 15, 25, """
<path d="M2.4 9.6a5.2 5.2 0 0 1 10.2 0z"/>
<path d="M2.4 9.6h10.2l-4.2 13a.9.9 0 0 1-1.8 0z"/>
<path d="M3.6 13.6h7.8M4.8 17.4h5.4"/>
""")
sym("burger", 24, 20, """
<path d="M2 7.4c0-3.2 4.4-5.8 10-5.8s10 2.6 10 5.8z"/>
<path d="M2.2 10.4h19.6M2.6 13.6c2.6 1.6 5 .4 7.6.4s5.4 1.2 8.4 0"/>
<path d="M2 15.6h20c0 1.8-1.6 3.2-3.6 3.2H5.6C3.6 18.8 2 17.4 2 15.6z"/>
""")
sym("book", 24, 20, """
<path d="M12 4.6C9.6 2.4 5.8 1.8 2 2.4v14c3.8-.6 7.6 0 10 2.2 2.4-2.2 6.2-2.8 10-2.2v-14c-3.8-.6-7.6 0-10 2.2z"/>
<path d="M12 4.6v14"/>
""")
sym("gamepad", 27, 18, """
<rect x="1.4" y="3.6" width="24.2" height="12.8" rx="5.6"/>
<path d="M7 7.8v4.4M4.8 10h4.4"/>
<circle cx="18.6" cy="8.6" r="1.2"/><circle cx="21.6" cy="11.4" r="1.2"/>
""")
sym("balloon", 16, 25, """
<path d="M8 1.6c3.6 0 6.4 3 6.4 6.8 0 4.2-3.4 7.6-6.4 9.4-3-1.8-6.4-5.2-6.4-9.4C1.6 4.6 4.4 1.6 8 1.6z"/>
<path d="M8 17.8v1.6M6.6 19.4h2.8l-1.4 4"/>
""")
sym("clock", 20, 20, """
<circle cx="10" cy="10" r="8.4"/><path d="M10 5.2V10l3.4 2.2"/>
""")
sym("bulb", 17, 23, """
<path d="M8.5 1.6a6.6 6.6 0 0 1 4 11.8c-.8.6-1.2 1.4-1.2 2.4H5.7c0-1-.4-1.8-1.2-2.4a6.6 6.6 0 0 1 4-11.8z"/>
<path d="M5.7 18.2h5.6M6.6 21h3.8"/>
""")
sym("cat", 21, 19, """
<path d="M3.4 6.4 2 1.8l4.6 2.6a9.6 9.6 0 0 1 7.8 0L19 1.8l-1.4 4.6a7.8 7.8 0 1 1-14.2 0z"/>
<circle cx="7.4" cy="10.4" r="1"/><circle cx="13.6" cy="10.4" r="1"/>
<path d="M10.5 13v1.4M8.4 15.4c1.4 1 2.8 1 4.2 0"/>
""")
sym("leaf", 20, 20, """
<path d="M18.4 1.6C9.6 1.6 2.4 5.6 2.4 12.4c0 2.4 1 4.4 2.6 5.8C10.6 18 18.4 13.6 18.4 1.6z"/>
<path d="M2.6 18.4C6.4 13.6 10.6 10.4 15 8.6"/>
""")
sym("gift", 21, 20, """
<rect x="1.6" y="7.4" width="17.8" height="11.4" rx="1.6"/>
<path d="M1 7.4h19.4M10.5 7.4v11.4"/>
<path d="M10.5 7.4C9 5 7.4 1.6 5 1.6a2.4 2.4 0 0 0 0 4.8h11a2.4 2.4 0 0 0 0-4.8c-2.4 0-4 3.4-5.5 5.8z"/>
""")
sym("sun", 22, 22, """
<circle cx="11" cy="11" r="5"/>
<path d="M11 1.6v2.6M11 17.8v2.6M1.6 11h2.6M17.8 11h2.6M4.4 4.4l1.8 1.8M15.8 15.8l1.8 1.8M17.6 4.4l-1.8 1.8M6.2 15.8l-1.8 1.8"/>
""")

sym("prompt", 26, 20, """
<rect x="1.4" y="1.6" width="23.2" height="16.8" rx="2.6"/>
<path d="M5.6 7.2 8.8 10l-3.2 2.8M11 13h6.4"/>
""")
sym("keyboard", 30, 18, """
<rect x="1.4" y="2.6" width="27.2" height="12.8" rx="2.4"/>
<path d="M5 6.4h1.6M9 6.4h1.6M13 6.4h1.6M17 6.4h1.6M21 6.4h1.6M25 6.4h.6"/>
<path d="M5 9.8h1.6M9 9.8h1.6M13 9.8h1.6M17 9.8h1.6M21 9.8h1.6"/>
<path d="M9 12.8h12"/>
""")
sym("mouse", 15, 23, """
<rect x="1.4" y="1.6" width="12.2" height="19.8" rx="6.1"/>
<path d="M7.5 5v4.4"/>
""")
sym("monitor", 26, 22, """
<rect x="1.4" y="1.6" width="23.2" height="15.4" rx="2.4"/>
<path d="M9.6 20.4h6.8M13 17v3.4"/>
""")
sym("server", 22, 22, """
<rect x="1.4" y="1.6" width="19.2" height="6.6" rx="1.6"/>
<rect x="1.4" y="10.4" width="19.2" height="6.6" rx="1.6"/>
<circle cx="5" cy="4.9" r="1"/><circle cx="5" cy="13.7" r="1"/>
<path d="M9.4 4.9h7.6M9.4 13.7h7.6M6 19.4h10"/>
""")
sym("database", 20, 23, """
<ellipse cx="10" cy="4.6" rx="8.4" ry="3.2"/>
<path d="M1.6 4.6v13.4c0 1.8 3.8 3.2 8.4 3.2s8.4-1.4 8.4-3.2V4.6"/>
<path d="M1.6 11.4c0 1.8 3.8 3.2 8.4 3.2s8.4-1.4 8.4-3.2"/>
""")
sym("usb", 13, 25, """
<rect x="2.6" y="7.4" width="7.8" height="16" rx="1.6"/>
<path d="M4.6 7.4V2.6h4v4.8M6.5 12v7"/>
""")
sym("floppy", 21, 21, """
<path d="M1.6 3.2A1.6 1.6 0 0 1 3.2 1.6h12.4l3.8 3.8v14a1.6 1.6 0 0 1-1.6 1.6H3.2a1.6 1.6 0 0 1-1.6-1.6z"/>
<path d="M6 1.6v6.2h8V1.6M6 19.4v-6.6h9v6.6"/>
""")
sym("wifi", 24, 18, """
<path d="M2.2 6.2a15.2 15.2 0 0 1 19.6 0"/>
<path d="M5.8 10a10 10 0 0 1 12.4 0"/>
<path d="M9.4 13.6a5 5 0 0 1 5.2 0"/>
<circle cx="12" cy="16.4" r="1.2"/>
""")
sym("network", 24, 22, """
<circle cx="12" cy="3.6" r="2.6"/><circle cx="3.6" cy="18" r="2.6"/><circle cx="20.4" cy="18" r="2.6"/>
<path d="M10.6 5.8 4.8 15.6M13.4 5.8l5.8 9.8M6.2 18h11.6"/>
""")
sym("brackets", 24, 18, """
<path d="M8.4 3.6 2.6 9l5.8 5.4M15.6 3.6 21.4 9l-5.8 5.4M13.4 2.4 10.6 15.6"/>
""")
sym("braces", 22, 20, """
<path d="M8.4 1.8c-2.6 0-2.6 3.4-2.6 5.4S4 10 2.6 10c1.4 0 3.2.8 3.2 2.8s0 5.4 2.6 5.4"/>
<path d="M13.6 1.8c2.6 0 2.6 3.4 2.6 5.4S18 10 19.4 10c-1.4 0-3.2.8-3.2 2.8s0 5.4-2.6 5.4"/>
""")
sym("lock", 18, 22, """
<rect x="1.6" y="9" width="14.8" height="11.4" rx="2.4"/>
<path d="M5 9V6.4a4 4 0 0 1 8 0V9"/><circle cx="9" cy="14.6" r="1.4"/>
""")
sym("key", 24, 15, """
<circle cx="5.6" cy="7.4" r="4"/>
<path d="M9.6 7.4h12.8M18.6 7.4v3.6M14.6 7.4v3"/>
""")
sym("shield", 19, 23, """
<path d="M9.5 1.6 17.4 4v8c0 4.6-3.2 8-7.9 9.4C4.8 20 1.6 16.6 1.6 12V4z"/>
<path d="M6.2 11.6 8.8 14l4.4-4.6"/>
""")
sym("battery", 26, 15, """
<rect x="1.4" y="2.6" width="20.2" height="9.8" rx="2.2"/>
<path d="M23.6 6v3M4.6 5.6h3.2v3.8H4.6zM9.8 5.6H13v3.8H9.8z"/>
""")
sym("plug", 18, 23, """
<path d="M4.4 8.4h9.2v3.8a4.6 4.6 0 0 1-9.2 0z"/>
<path d="M6.4 8.4V2.6M11.6 8.4V2.6M9 16.8v4.6"/>
""")
sym("robot", 22, 21, """
<rect x="2.6" y="5.4" width="16.8" height="12.6" rx="3.2"/>
<circle cx="8" cy="11.4" r="1.4"/><circle cx="14" cy="11.4" r="1.4"/>
<path d="M11 1.6v3.8M1 10v4M21 10v4M9 15.4h4"/>
""")
sym("satellite", 24, 22, """
<circle cx="12" cy="12.8" r="3.6"/>
<path d="M12 9.2V3.6M12 16.4V21M8.4 12.8H2.6M15.6 12.8h5.8"/>
<path d="M6.6 7.4 9.4 10.2M17.4 7.4 14.6 10.2M6.6 18.2l2.8-2.8M17.4 18.2l-2.8-2.8"/>
""")
sym("infinity", 28, 15, """
<path d="M14 7.4c2-3.4 3.6-5 6-5a5 5 0 0 1 0 10c-2.4 0-4-1.6-6-5-2-3.4-3.6-5-6-5a5 5 0 0 0 0 10c2.4 0 4-1.6 6-5z"/>
""")
sym("box", 22, 22, """
<path d="M11 1.6 20.4 6v10L11 20.4 1.6 16V6z"/>
<path d="M1.6 6 11 10.4 20.4 6M11 10.4v10"/>
""")
sym("puzzle", 22, 21, """
<path d="M2.6 8.4h4a2.4 2.4 0 1 1 4.8 0h4v4a2.4 2.4 0 1 1 0 4.8v.8H2.6v-4.4a2.4 2.4 0 1 0 0-4.8z"/>
""")
sym("cursor", 15, 22, """
<path d="M2.4 1.6 13 12.4l-4.6.6 2.8 5.6-2.4 1.2-2.8-5.6-3 3.4z"/>
""")
sym("terminal-cursor", 20, 20, """
<path d="M2.6 4.4 7 10l-4.4 5.6M10 15.6h7.4"/>
""")
sym("copyleft", 20, 20, """
<circle cx="10" cy="10" r="8.4"/>
<path d="M7.4 7.2a4 4 0 1 0 0 5.6M12.6 6.6v6.8"/>
""")
sym("hdd", 24, 18, """
<rect x="1.4" y="2.6" width="21.2" height="12.8" rx="2.4"/>
<circle cx="12" cy="9" r="4"/><circle cx="12" cy="9" r="1"/>
<path d="M18.4 12.6h1.6"/>
""")

sym("soccer", 21, 21, """
<circle cx="10.5" cy="10.5" r="9"/>
<path d="M10.5 5.4 14.4 8.2 12.9 12.8H8.1L6.6 8.2z"/>
<path d="M10.5 1.5v3.9M2.4 8.2l4.2 0M18.6 8.2l-4.2 0M5.6 18.2l2.5-5.4M15.4 18.2l-2.5-5.4"/>
""")
sym("basketball", 21, 21, """
<circle cx="10.5" cy="10.5" r="9"/>
<path d="M10.5 1.5v18M1.5 10.5h18M4.2 4.2c4 3.4 4 8.6 0 12.6M16.8 4.2c-4 3.4-4 8.6 0 12.6"/>
""")
sym("racket", 17, 26, """
<ellipse cx="8.5" cy="8.4" rx="6.8" ry="7.6"/>
<path d="M3.4 4.4c4 2.6 6.6 6 8.2 9.8M13.6 4.4c-4 2.6-6.6 6-8.2 9.8"/>
<path d="M8.5 16v8.4"/>
""")
sym("dumbbell", 27, 16, """
<rect x="1.4" y="4.6" width="4" height="6.8" rx="1.4"/><rect x="21.6" y="4.6" width="4" height="6.8" rx="1.4"/>
<path d="M5.4 8h16.2M6.8 3.4v9.2M20.2 3.4v9.2"/>
""")
sym("trophy", 22, 23, """
<path d="M5.6 1.6h10.8v6.2a5.4 5.4 0 0 1-10.8 0z"/>
<path d="M5.6 3.4H2.4v1.8a3.4 3.4 0 0 0 3.2 3.4M16.4 3.4h3.2v1.8a3.4 3.4 0 0 1-3.2 3.4"/>
<path d="M11 13.2v4.2M7 21.4h8l-.8-4H7.8z"/>
""")
sym("medal", 18, 25, """
<circle cx="9" cy="16.6" r="6.4"/><path d="M9 13.4l1.2 2.4 2.6.4-1.9 1.8.5 2.6L9 19.4l-2.4 1.2.5-2.6-1.9-1.8 2.6-.4z"/>
<path d="M4.6 1.6 7 10M13.4 1.6 11 10"/>
""")
sym("stopwatch", 19, 23, """
<circle cx="9.5" cy="13.4" r="7.8"/><path d="M9.5 8.8v4.6l3 2"/>
<path d="M6.6 1.6h5.8M9.5 1.6v3.8M15.4 5.2l2-2"/>
""")
sym("skate", 26, 15, """
<path d="M2.6 4.4c3-2 17.8-2 20.8 0 1.4 1 .4 2.6-1.6 2.8L5.8 7.2C3.4 7.2 1.2 5.4 2.6 4.4z"/>
<circle cx="7.4" cy="11.4" r="2.6"/><circle cx="18.6" cy="11.4" r="2.6"/>
""")
sym("target", 21, 21, """
<circle cx="10.5" cy="10.5" r="9"/><circle cx="10.5" cy="10.5" r="5.4"/><circle cx="10.5" cy="10.5" r="1.6"/>
""")
sym("dice", 20, 20, """
<rect x="1.6" y="1.6" width="16.8" height="16.8" rx="3.4"/>
<circle cx="6.4" cy="6.4" r="1.2"/><circle cx="13.6" cy="13.6" r="1.2"/><circle cx="10" cy="10" r="1.2"/>
""")
sym("joystick", 20, 24, """
<circle cx="10" cy="4.6" r="3"/><path d="M10 7.6v7"/>
<path d="M2.6 14.6h14.8a2 2 0 0 1 2 2v4a2 2 0 0 1-2 2H2.6a2 2 0 0 1-2-2v-4a2 2 0 0 1 2-2z"/>
""")
sym("arcade", 20, 25, """
<rect x="1.6" y="1.6" width="16.8" height="21.8" rx="3"/>
<rect x="4.6" y="4.6" width="10.8" height="7.8" rx="1.4"/>
<circle cx="6.6" cy="17" r="1.4"/><circle cx="12.4" cy="17" r="1.4"/><path d="M5.4 20.8h9"/>
""")
sym("invader", 22, 19, """
<path d="M4.6 4.6h2.8V1.8h7.2v2.8h2.8v3h2.8v7.6h-2.8v-3h-2.8v3h-7.2v-3H4.6v3H1.8V7.6h2.8z"/>
<circle cx="8.4" cy="8.4" r="1"/><circle cx="13.6" cy="8.4" r="1"/>
""")
sym("cards", 24, 22, """
<rect x="7.4" y="3.6" width="12" height="16.8" rx="2" transform="rotate(8 13.4 12)"/>
<rect x="2.6" y="1.6" width="12" height="16.8" rx="2" transform="rotate(-10 8.6 10)"/>
<path d="M8.6 6.6 10.8 9 8.6 11.4 6.4 9z"/>
""")
sym("palette", 24, 21, """
<path d="M12 1.6c6 0 10.4 3.6 10.4 8 0 3-2.6 4-4.6 4h-2c-1.6 0-2.6 1-2.6 2.2 0 1.6 1.4 1.8 1.4 3 0 .8-1 1.6-2.6 1.6C5.6 20.4 1.6 16 1.6 10.4 1.6 5 6 1.6 12 1.6z"/>
<circle cx="7" cy="8.6" r="1.4"/><circle cx="11.4" cy="5.6" r="1.4"/><circle cx="16.4" cy="7.4" r="1.4"/>
""")
sym("brush", 15, 25, """
<path d="M4.4 1.6h6.2v11.8H4.4z"/>
<path d="M3 13.4h9v3.6c0 3.6-1.6 6.4-4.5 6.4S3 20.6 3 17z"/>
""")
sym("pencil", 22, 22, """
<path d="M15.6 1.8 20.2 6.4 7.6 19H3v-4.6z"/>
<path d="M13.4 4 18 8.6"/>
""")
sym("clapper", 25, 20, """
<path d="M1.6 7.4h21.8v10.8a1.6 1.6 0 0 1-1.6 1.6H3.2a1.6 1.6 0 0 1-1.6-1.6z"/>
<path d="M2.4 7.4 4.6 2.2l4.6 1.6-2.2 3.6M9.2 3.8l4.6 1.6-2.2 2M13.8 5.4l4.6 1.6-1.6 .4"/>
""")
sym("mic", 15, 25, """
<rect x="4.4" y="1.6" width="6.4" height="12.8" rx="3.2"/>
<path d="M2.2 12.4a5.4 5.4 0 0 0 10.8 0M7.6 17.8v5.6M4.8 23.4h5.6"/>
""")
sym("piano", 24, 18, """
<rect x="1.6" y="2.6" width="20.8" height="12.8" rx="1.8"/>
<path d="M8.5 2.6v12.8M15.5 2.6v12.8M5 2.6v7.4h2.4V2.6M12 2.6v7.4h2.4V2.6M19 2.6v7.4h-2.4"/>
""")
sym("mask", 24, 19, """
<path d="M12 1.8c5 0 10.4 1 10.4 4.4 0 4.6-3.6 11.4-10.4 11.4S1.6 10.8 1.6 6.2c0-3.4 5.4-4.4 10.4-4.4z"/>
<circle cx="8" cy="7.6" r="1.2"/><circle cx="16" cy="7.6" r="1.2"/><path d="M8.6 12.4c2 1.6 4.8 1.6 6.8 0"/>
""")
sym("frame", 22, 22, """
<rect x="1.6" y="1.6" width="18.8" height="18.8" rx="1.6"/>
<path d="M4.4 15.4 8.6 10l3.4 3.6 2.6-2.4 3 4.2z"/><circle cx="14.6" cy="6.6" r="1.4"/>
""")
sym("hat", 24, 18, """
<path d="M6.6 11.4V4.6a2.6 2.6 0 0 1 2.6-2.6h5.6a2.6 2.6 0 0 1 2.6 2.6v6.8"/>
<path d="M1.6 12.4c0-1 2-1.6 4.4-2h12c2.4.4 4.4 1 4.4 2 0 1.8-4.6 3.4-10.4 3.4S1.6 14.2 1.6 12.4z"/>
""")
sym("glasses", 28, 13, """
<circle cx="6.6" cy="7.4" r="5"/><circle cx="21.4" cy="7.4" r="5"/>
<path d="M11.6 6.4c1.6-1 3.2-1 4.8 0M1.6 5.4 3.4 3.4M26.4 5.4 24.6 3.4"/>
""")
sym("shirt", 25, 22, """
<path d="M9 1.8 3 4.6 1.6 9.4l3.6 1.4v9.4h14.6v-9.4l3.6-1.4L22 4.6 16 1.8"/>
<path d="M9 1.8c0 2 1.4 3 3.5 3s3.5-1 3.5-3"/>
""")
sym("sneaker", 27, 15, """
<path d="M1.6 12.4V6.6l4.4-2 3 2.4 3.6-.6 5.4 3.2 6.4 1.4c1.6.4 2 1.6 2 3.2H1.6z"/>
<path d="M6 4.6 8.4 8M12 6.4l1.6 2.4"/>
""")
sym("umbrella", 24, 24, """
<path d="M1.6 11.4a10.4 10.4 0 0 1 20.8 0c-1.8-1.6-3.6-1.6-5.2 0-1.6-1.6-3.4-1.6-5.2 0-1.8-1.6-3.6-1.6-5.2 0-1.6-1.6-3.4-1.6-5.2 0z"/>
<path d="M12 11.4v9a2.4 2.4 0 0 1-4.8 0"/>
""")
sym("suitcase", 24, 21, """
<rect x="1.6" y="5.6" width="20.8" height="13.8" rx="2.4"/>
<path d="M8 5.6V3.4a1.8 1.8 0 0 1 1.8-1.8h4.4A1.8 1.8 0 0 1 16 3.4v2.2M1.6 12.4h20.8"/>
""")
sym("chair", 19, 24, """
<path d="M3.4 1.6h12.2v9.8H3.4z"/>
<path d="M2 11.4h15M4.4 11.4v11M14.6 11.4v11M4.4 17h10.2"/>
""")
sym("candle", 15, 25, """
<rect x="4.4" y="8.4" width="6.2" height="12.8" rx="1.4"/>
<path d="M2.6 21.2h9.8M7.5 8.4V5.6"/>
<path d="M7.5 1.6c1.6 1.4 2.4 2.4 2.4 3.4a2.4 2.4 0 0 1-4.8 0c0-1 .8-2 2.4-3.4z"/>
""")
sym("plant", 19, 24, """
<path d="M5 14.4h9l-1 8H6z"/>
<path d="M9.5 14.4V7M9.5 9.4C7 9.4 4.4 7.4 4.4 3.6c3.6 0 5.1 2.4 5.1 5.8zM9.5 11.4c2.5 0 5.1-2 5.1-5.8-3.6 0-5.1 2.4-5.1 5.8z"/>
""")
sym("wineglass", 15, 25, """
<path d="M3.4 1.6h8.2v5.4a4.1 4.1 0 0 1-8.2 0z"/>
<path d="M7.5 11.4v10M4.4 21.4h6.2"/>
""")
sym("popcorn", 20, 24, """
<path d="M4.6 9.4h10.8l-1.4 13H6z"/>
<circle cx="6" cy="6.4" r="2.6"/><circle cx="10" cy="4.6" r="2.8"/><circle cx="14" cy="6.6" r="2.4"/>
""")
sym("dog", 22, 20, """
<path d="M4.4 5.4 2.6 1.8l4 1.6a9 9 0 0 1 8.8 0l4-1.6-1.8 3.6a7.4 7.4 0 1 1-13.2 0z"/>
<circle cx="8" cy="9.6" r="1"/><circle cx="14" cy="9.6" r="1"/>
<path d="M11 12.4v1.4M9 15.4c1.4 1 2.6 1 4 0"/>
""")
sym("bird", 22, 18, """
<path d="M1.6 9.4c0-3.4 3-6.2 6.6-6.2 3 0 4.6 1.4 6 3.2l6.2-1-3.4 3.6c0 4-3.2 7.4-7.2 7.4-4.4 0-8.2-2.8-8.2-7z"/>
<circle cx="7" cy="7.6" r="1"/>
""")
sym("fish", 24, 16, """
<path d="M1.6 8c3-3.6 6.6-5.4 10.4-5.4 4.4 0 7.6 2.4 9.4 5.4-1.8 3-5 5.4-9.4 5.4-3.8 0-7.4-1.8-10.4-5.4z"/>
<path d="M21.4 8 22.4 3.4 18.6 6M21.4 8l1 4.6-3.8-2.6"/><circle cx="7.6" cy="7" r="1"/>
""")
sym("paw", 19, 19, """
<ellipse cx="4.4" cy="7.4" rx="2.4" ry="3"/><ellipse cx="9.5" cy="4.6" rx="2.4" ry="3.2"/><ellipse cx="14.6" cy="7.4" rx="2.4" ry="3"/>
<path d="M9.5 10.4c3.4 0 5.6 2 5.6 4.2s-2.4 2.8-5.6 2.8-5.6-.6-5.6-2.8 2.2-4.2 5.6-4.2z"/>
""")
sym("compass", 21, 21, """
<circle cx="10.5" cy="10.5" r="9"/>
<path d="M14 7 8.6 8.6 7 14l5.4-1.6z"/>
""")
sym("map", 24, 20, """
<path d="M1.6 4.6 8.6 1.8v13.6l-7 2.8zM8.6 1.8l6.8 2.8v13.6L8.6 15.4zM15.4 4.6l7-2.8v13.6l-7 2.8z"/>
""")
sym("ticket", 26, 17, """
<path d="M1.6 4.4a1.6 1.6 0 0 1 1.6-1.6h19.2a1.6 1.6 0 0 1 1.6 1.6v2.4a2.6 2.6 0 0 0 0 5.2v2.4a1.6 1.6 0 0 1-1.6 1.6H3.2a1.6 1.6 0 0 1-1.6-1.6v-2.4a2.6 2.6 0 0 0 0-5.2z"/>
<path d="M9.4 2.8v2M9.4 7.4v2M9.4 12v2"/>
""")
sym("clover", 20, 22, """
<path d="M10 10.4c-1.4-2.6-.6-4.8 1.4-5.6 2-.8 3.8.6 3.8 2.6 0 1.8-1.8 3-5.2 3z"/>
<path d="M10 10.4c1.4-2.6.6-4.8-1.4-5.6-2-.8-3.8.6-3.8 2.6 0 1.8 1.8 3 5.2 3z"/>
<path d="M10 10.4c-2.6 1.4-4.8.6-5.6-1.4M10 10.4c2.6 1.4 4.8.6 5.6-1.4M10 10.4v10.4"/>
""")
sym("balloon-gift", 20, 20, """
<path d="M10 1.8 12.4 7l5.8.8-4.2 4 1 5.8L10 15l-5 2.6 1-5.8-4.2-4L7.6 7z"/>
""")



TEXTURE = ["dot", "ring", "sparkle"]


def build(color, opacity):
    rng = random.Random(90210)
    placed, uses = [], []

    def far(x, y, r):
        for px, py, pr in placed:
            dx, dy = abs(x - px), abs(y - py)
            if math.hypot(min(dx, TILE - dx), min(dy, TILE - dy)) < r + pr:
                return False
        return True

    def try_place(r, tries):
        for _ in range(tries):
            x, y = rng.uniform(0, TILE), rng.uniform(0, TILE)
            if far(x, y, r):
                placed.append((x, y, r))
                return x, y
        return None

    def emit(name, x, y, size, angle, box):
        cx, cy, r = box
        s = size / (2 * r)
        for dx in (-TILE, 0, TILE):
            for dy in (-TILE, 0, TILE):
                nx, ny = x + dx, y + dy
                if -size < nx < TILE + size and -size < ny < TILE + size:
                    uses.append(
                        f'<use href="#s-{name}" transform="translate({nx:.1f} {ny:.1f}) '
                        f'rotate({angle:.1f}) scale({s:.4f}) '
                        f'translate({-cx:.3f} {-cy:.3f})"/>'
                    )

    drawings = [n for n in DOODLES if n not in TEXTURE]
    for name in rng.sample(drawings, len(drawings)):
        w, h, _ = DOODLES[name]
        for size in (rng.uniform(62, 80), 56, 50, 44, 38):
            spot = try_place(size / 2, tries=2500)
            if spot:
                emit(name, spot[0], spot[1], size, rng.uniform(-24, 24),
                     (w / 2, h / 2, math.hypot(w, h) / 2))
                break

    for lo, hi, count in [(20, 30, 90), (13, 20, 160), (8, 13, 260)]:
        for _ in range(count):
            size = rng.uniform(lo, hi)
            spot = try_place(size / 2, tries=500)
            if spot:
                n = rng.choice(TEXTURE)
                w, h, _ = DOODLES[n]
                emit(n, spot[0], spot[1], size, rng.uniform(-30, 30),
                     (w / 2, h / 2, math.hypot(w, h) / 2))

    line = (f'fill="none" stroke="currentColor" stroke-width="{STROKE}" '
            f'stroke-linejoin="round" stroke-linecap="round"')
    defs = []
    for n, (w, h, b) in DOODLES.items():
        defs.append(f'<g id="s-{n}" {line}>{non_scaling(b)}</g>')

    return f"""<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {TILE} {TILE}" width="{TILE}" height="{TILE}">
<defs>
{chr(10).join(defs)}
</defs>
<g color="{color}" opacity="{opacity}">
{chr(10).join(uses)}
</g>
</svg>
"""

base = os.path.join(os.path.dirname(os.path.abspath(__file__)),
                    "..", "..", "internal", "web", "static", "img") + os.sep
open(base + "wallpaper-dark.svg", "w").write(build("#ffffff", "0.92"))
open(base + "wallpaper-light.svg", "w").write(build("#6f6152", "0.17"))
print("ok:", len(DOODLES), "rabiscos")
