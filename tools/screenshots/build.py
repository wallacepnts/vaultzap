#!/usr/bin/env python3
"""Regenerates every screenshot under docs/img/{pt,en}/.

Local tool, not part of the build and not versioned (see .gitignore). It creates
fictional conversations, imports them into a throwaway instance, pins a couple of
messages so the pinned strip has real state, and photographs each screen.

    python3 tools/screenshots/build.py            # both languages
    python3 tools/screenshots/build.py pt         # one language

Needs: chromium, pngquant, Pillow, and a Go toolchain (the app is run from source).

The gallery, calendar, search, profile and merge panels have no navigable URL of
their own — they are htmx fragments. Instead of driving the browser, each page is
assembled: GET / for the real layout, GET the fragment with "HX-Request: true",
splice one into the other, add <base href> so the assets resolve.
"""

import datetime
import os
import re
import shutil
import signal
import socket
import subprocess
import sys
import sqlite3
import time
import urllib.request
import zipfile

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from fixtures import PT, EN, images, long_chat, block  # noqa: E402

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
# Wide enough for the three columns: with the right panel open at 1280 the conversation
# was squeezed to ~340px and the bubbles wrapped every two words.
WINDOW = "1500,860"

WORK = os.path.join(ROOT, "tools", "screenshots", ".work")
SHOTS = ["conversation", "gallery", "calendar", "search",
         "imports", "profile", "merge", "owner-picker", "pinned"]


def free_port():
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def build_inbox(lang, root):
    """Writes the three fictional units: a .zip with media, a long .txt, a group .txt."""
    shutil.rmtree(root, ignore_errors=True)
    os.makedirs(root)
    tmp = os.path.join(root, "_img")
    os.makedirs(tmp)
    imgs = images(tmp)
    pt = lang == "pt"
    fmt = "%d/%m/%Y %H:%M" if pt else "%m/%d/%Y %H:%M"
    attached = "(arquivo anexado)" if pt else "(file attached)"
    contact = "Ana Beatriz" if pt else "Emily Carter"

    if pt:
        day1 = [(contact, "Bom dia! Conseguiu ver o orçamento que mandei ontem?"),
                ("Wallace", "Vi sim, ficou ótimo. Só uma dúvida na parte da instalação"),
                (contact, "Pode falar"),
                ("Wallace", "O prazo de 15 dias é útil ou corrido?"),
                (contact, "Úteis. Se fechar essa semana, entrego antes do feriado"),
                ("Wallace", "Fechado. Manda o contrato que eu assino hoje"),
                (contact, f"{imgs[0]} {attached}"),
                (contact, "Essa é a foto do modelo que a gente conversou"),
                ("Wallace", "Perfeito, é exatamente esse")]
        day2 = [(contact, "Contrato assinado chegou aqui, obrigada!"),
                ("Wallace", "Show. Quando começam?"),
                (contact, "Segunda que vem, 8h em ponto"),
                (contact, f"{imgs[1]} {attached}"), (contact, f"{imgs[2]} {attached}"),
                (contact, "Fotos do galpão pra você ver o espaço"),
                ("Wallace", "Perfeito 👍")]
        day3 = [(contact, f"{imgs[i]} {attached}") for i in range(3, 8)] + [
                (contact, "Primeiro dia de obra, mandei cinco de uma vez"),
                ("Wallace", "Ficou melhor do que eu imaginava"),
                ("Wallace", "Manda o link daquele fornecedor de novo?"),
                (contact, "https://exemplo.com.br/catalogo/perfis-de-aluminio"),
                ("Wallace", "Valeu! Vou pedir os perfis hoje ainda"),
                (contact, "Combinado. Qualquer coisa me chama 😊"),
                ("Wallace", "Fechou, Ana. Obrigado!")]
        group = [("Ana Beatriz", "Pessoal, criei o grupo pra combinar a mudança"),
                 ("Rafael", "Boa! Eu consigo ajudar no sábado de manhã"),
                 ("Carla", "Eu só depois das 10h"),
                 ("Wallace", "Por mim tudo bem. Alguém tem uma van?"),
                 ("Rafael", "Meu cunhado tem, vou perguntar"),
                 ("Carla", "Levo as caixas que sobraram da última vez"),
                 ("Ana Beatriz", "Fechou. Sábado 9h na portaria"),
                 ("Rafael", "👍"), ("Wallace", "Combinado")]
        names = ("Conversa do WhatsApp com Ana Beatriz.zip",
                 "Conversa do WhatsApp com Marcos Andrade.txt",
                 "Conversa do WhatsApp com Mudança do escritório.txt")
        cfg, pair = PT, ("Marcos", "Wallace")
    else:
        day1 = [(contact, "Morning! Did you get a chance to look at the quote I sent yesterday?"),
                ("Wallace", "I did, it looks great. Just one question about the installation"),
                (contact, "Go ahead"),
                ("Wallace", "Is the 15-day deadline business days or calendar days?"),
                (contact, "Business days. If we close this week I can deliver before the holiday"),
                ("Wallace", "Let's do it. Send the contract over and I'll sign today"),
                (contact, f"{imgs[0]} {attached}"),
                (contact, "That's the model we talked about"),
                ("Wallace", "Perfect, that's exactly the one")]
        day2 = [(contact, "Signed contract just arrived, thank you!"),
                ("Wallace", "Great. When do you start?"),
                (contact, "Next Monday, 8am sharp"),
                (contact, f"{imgs[1]} {attached}"), (contact, f"{imgs[2]} {attached}"),
                (contact, "Photos of the warehouse so you can see the space"),
                ("Wallace", "Perfect 👍")]
        day3 = [(contact, f"{imgs[i]} {attached}") for i in range(3, 8)] + [
                (contact, "First day on site, sending five at once"),
                ("Wallace", "It turned out better than I imagined"),
                ("Wallace", "Can you send me that supplier link again?"),
                (contact, "https://example.com/catalogue/aluminium-profiles"),
                ("Wallace", "Thanks! I'll order the profiles today"),
                (contact, "Sounds good. Give me a shout if anything comes up 😊"),
                ("Wallace", "Will do, Emily. Thank you!")]
        group = [("Emily Carter", "Hey all, made this group to sort out the move"),
                 ("Rafael", "Nice! I can help on Saturday morning"),
                 ("Claire", "I'm only free after 10"),
                 ("Wallace", "Works for me. Does anyone have a van?"),
                 ("Rafael", "My brother-in-law does, I'll ask"),
                 ("Claire", "I'll bring the boxes left over from last time"),
                 ("Emily Carter", "Settled. Saturday 9am at the entrance"),
                 ("Rafael", "👍"), ("Wallace", "See you there")]
        names = ("WhatsApp Chat with Emily Carter.zip",
                 "WhatsApp Chat with Marcus Reed.txt",
                 "WhatsApp Chat with Office move.txt")
        cfg, pair = EN, ("Marcus", "Wallace")

    lines = (block(day1, datetime.datetime(2026, 3, 2, 8, 12), fmt)
             + block(day2, datetime.datetime(2026, 3, 9, 14, 3), fmt)
             + block(day3, datetime.datetime(2026, 3, 16, 10, 25), fmt))
    txt = os.path.join(tmp, "_chat.txt")
    open(txt, "w").write("\n".join(lines) + "\n")
    with zipfile.ZipFile(os.path.join(root, names[0]), "w", zipfile.ZIP_DEFLATED) as z:
        z.write(txt, "_chat.txt")
        for f in imgs:
            z.write(os.path.join(tmp, f), f)
    open(os.path.join(root, names[1]), "w").write("\n".join(
        long_chat(cfg, *pair, 420, datetime.datetime(2025, 11, 3, 9, 17), fmt)) + "\n")
    open(os.path.join(root, names[2]), "w").write("\n".join(
        block(group, datetime.datetime(2026, 4, 11, 19, 40), fmt)) + "\n")
    shutil.rmtree(tmp)


def start_app(inbox, data, port, date_order):
    """Runs the app from source. VAULTZAP_ME is deliberately unset: the 1:1 chats get
    their owner inferred, and the group stays without one — which is what makes the
    "which of these is you?" bar show up in owner-picker.png."""
    env = dict(os.environ,
               VAULTZAP_ADDR=f"127.0.0.1:{port}",
               VAULTZAP_DB=os.path.join(data, "vaultzap.db"),
               VAULTZAP_MEDIA_DIR=os.path.join(data, "media"),
               VAULTZAP_INBOX=inbox,
               VAULTZAP_AFTER_IMPORT="keep",
               VAULTZAP_DATE_ORDER=date_order)
    env.pop("VAULTZAP_ME", None)
    os.makedirs(data, exist_ok=True)
    proc = subprocess.Popen(["go", "run", "./cmd/vaultzap"], cwd=ROOT, env=env,
                            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                            start_new_session=True)
    base = f"http://127.0.0.1:{port}"
    for _ in range(90):
        time.sleep(1)
        try:
            urllib.request.urlopen(base + "/healthz", timeout=2)
            return proc, base
        except Exception:
            continue
    stop_app(proc)
    raise SystemExit("the app did not come up")


def stop_app(proc):
    os.killpg(os.getpgid(proc.pid), signal.SIGTERM)
    proc.wait(timeout=30)


def wait_for_import(db, expected, timeout=120):
    for _ in range(timeout):
        try:
            rows = list(db.execute("select id, is_group, message_count from chats"))
        except sqlite3.OperationalError:
            rows = []
        if len(rows) >= expected:
            return rows
        time.sleep(1)
    raise SystemExit(f"only {len(rows)} of {expected} conversations were imported")


class Shooter:
    def __init__(self, base, locale, dest):
        self.base, self.locale, self.dest = base, locale, dest
        os.makedirs(dest, exist_ok=True)

    def get(self, path, fragment=False, post=False):
        req = urllib.request.Request(self.base + path, data=b"" if post else None)
        req.add_header("Cookie", "locale=" + self.locale)
        if fragment:
            req.add_header("HX-Request", "true")
        if post:
            req.add_header("Origin", self.base)
        return urllib.request.urlopen(req).read().decode()

    def shoot(self, name, conversation, panel=None):
        page = self.get("/")
        page = page.replace("<head>", f'<head><base href="{self.base}/">', 1)
        page = re.sub(r'<main id="conversation">.*?</main>',
                      lambda m: '<main id="conversation">' + self.get(conversation, fragment=True) + '</main>',
                      page, count=1, flags=re.S)
        if panel:
            page = page.replace('<aside id="right-panel"></aside>',
                                '<aside id="right-panel">' + self.get(panel, fragment=True) + '</aside>', 1)
        page = page.replace("</body>", '<script>window.addEventListener("load",()=>'
                            '{const b=document.getElementById("body-conversation");'
                            'if(b) b.scrollTop=b.scrollHeight;});</script></body>', 1)
        html = os.path.join(WORK, name + ".html")
        open(html, "w").write(page)
        png = os.path.join(self.dest, name + ".png")
        subprocess.run(["chromium", "--headless", "--disable-gpu", "--hide-scrollbars",
                        f"--window-size={WINDOW}", "--virtual-time-budget=5000",
                        f"--screenshot={png}", "file://" + html], capture_output=True)
        subprocess.run(["pngquant", "--quality=70-92", "--speed", "1", "--force",
                        "--output", png, png], capture_output=True)
        print(f"  {name}.png  {os.path.getsize(png) // 1024} KB")


def run(lang):
    locale = "pt-BR" if lang == "pt" else "en"
    inbox = os.path.join(WORK, f"inbox-{lang}")
    data = os.path.join(WORK, f"data-{lang}")
    shutil.rmtree(data, ignore_errors=True)
    build_inbox(lang, inbox)
    proc, base = start_app(inbox, data, free_port(), "DMY" if lang == "pt" else "MDY")
    try:
        db = sqlite3.connect(os.path.join(data, "vaultzap.db"))
        # /healthz answers before the startup scan finishes — it takes two passes with a
        # gap between them, by design (see §5.9). Wait for the three units to land.
        rows = wait_for_import(db, 3)
        long_id = next(i for i, g, c in rows if c > 300)
        contact_id = next(i for i, g, c in rows if not g and c <= 300)
        group_id = next(i for i, g, c in rows if g)
        month = db.execute("select substr(sent_at,1,7) m, count(*) c from messages "
                           "where chat_id=? group by m order by c desc limit 1",
                           (long_id,)).fetchone()[0]
        pinnable = [r[0] for r in db.execute(
            "select id from messages where chat_id=? and kind='text' order by sent_at desc limit 2",
            (long_id,))]

        s = Shooter(base, locale, os.path.join(ROOT, "docs", "img", lang))
        print(f"{lang}:")
        s.shoot("conversation", f"/chats/{contact_id}")
        s.shoot("gallery", f"/chats/{contact_id}/media?aba=fotos")
        s.shoot("calendar", f"/chats/{long_id}", f"/chats/{long_id}/calendario?mes={month}")
        term = "orcamento" if lang == "pt" else "quote"
        s.shoot("search", f"/chats/{long_id}", f"/chats/{long_id}/search?q={term}")
        s.shoot("imports", "/imports")
        s.shoot("profile", f"/chats/{contact_id}", f"/chats/{contact_id}/perfil")
        s.shoot("merge", f"/chats/{contact_id}", f"/chats/{contact_id}/mesclar")
        s.shoot("owner-picker", f"/chats/{group_id}")
        for msg in pinnable:
            s.get(f"/chats/{long_id}/messages/{msg}/fixar", fragment=True, post=True)
        s.shoot("pinned", f"/chats/{long_id}")
    finally:
        stop_app(proc)


if __name__ == "__main__":
    os.makedirs(WORK, exist_ok=True)
    for lang in (sys.argv[1:] or ["pt", "en"]):
        run(lang)
