"""Fictional conversation material for the screenshots (see build.py)."""
import datetime, random, os
from PIL import Image, ImageDraw, ImageFilter

def images(dest, n=8):
    r=random.Random(3); nomes=[]
    cores=[((38,92,120),(120,190,170)),((150,110,70),(230,200,150)),((60,70,110),(150,120,180)),
           ((40,110,90),(180,200,120)),((120,60,60),(220,170,120)),((30,60,90),(120,160,200)),
           ((90,90,40),(200,190,120)),((70,40,90),(170,130,200))]
    for i,(c1,c2) in enumerate(cores[:n], start=1):
        w,h=(1200,900) if i%3 else (1000,1000)
        img=Image.new("RGB",(w,h),c1); d=ImageDraw.Draw(img)
        for y in range(h):
            f=y/h; d.line([(0,y),(w,y)], fill=tuple(int(c1[k]+(c2[k]-c1[k])*f) for k in range(3)))
        for _ in range(7):
            x,y=r.randint(0,w),r.randint(0,h); rad=r.randint(30,110)
            d.ellipse([x-rad,y-rad,x+rad,y+rad], fill=tuple(min(255,c+r.randint(-25,25)) for c in c2))
        img=img.filter(ImageFilter.GaussianBlur(9))
        nome=f"IMG-2026030{i}-WA000{i}.jpg"; img.save(os.path.join(dest,nome), quality=88); nomes.append(nome)
    return nomes

PT = dict(
  pares=[("Falei com {quem} sobre {coisa}","Ele disse que resolve até {dia}"),
         ("Consegui {acao} {coisa}","Ficou pra {dia}"),
         ("{coisa} chegou hoje de manhã","Confere quando puder"),
         ("Já pedi {coisa}","Prometeram entregar até {dia}"),
         ("Adiantei {coisa}","Sobrou tempo pra {acao} o resto")],
  soltas=["Vou chegar uns {n} minutos atrasado","Te ligo assim que sair da reunião",
    "Marquei {coisa} pra {dia}","Reservei a sala pra {dia}, {n} lugares",
    "O orçamento {dequem} veio {n}% mais barato","Mandei o orçamento revisado {dequem}",
    "Ainda espero o orçamento {dequem}","O orçamento {dequem} foi aprovado",
    "Anotei aqui, pode deixar","Manda o endereço quando puder",
    "Confirmei com {quem} por telefone agora","{coisa} precisa de duas assinaturas",
    "Achei um lugar melhor, e mais perto","{coisa} atrasou de novo, falei com {quem}",
    "Deu tudo certo com {coisa}","Consegui trocar o horário {dequem}",
    "Vamos precisar de mais {n} caixas","Fiz o pagamento {dequem} hoje de manhã"],
  quem=["o fornecedor","a transportadora","o contador","a Marina","o pessoal da obra","o síndico","a loja"],
  coisa=["o contrato","a papelada","as chaves","o relatório","a nota fiscal","o material","a entrega",
         "o pedido","a reforma","o seguro","a mudança","o projeto"],
  acao=["adiantar","fechar","revisar","separar","organizar","conferir"],
  dia=["segunda","terça","quarta","quinta","sexta","o fim do mês","a semana que vem"],
  dequem=["da elétrica","da pintura","do frete","do escritório","da obra"])

EN = dict(
  pares=[("Spoke to {quem} about {coisa}","They said it'll be sorted by {dia}"),
         ("Managed to {acao} {coisa}","Moved it to {dia}"),
         ("{coisa} arrived this morning","Have a look when you can"),
         ("I've ordered {coisa}","They promised it by {dia}"),
         ("Got {coisa} out of the way early","Left us time to {acao} the rest")],
  soltas=["I'll be about {n} minutes late","I'll call you as soon as I'm out of the meeting",
    "Booked {coisa} for {dia}","Reserved the room for {dia}, {n} seats",
    "The {dequem} quote came in {n}% cheaper","Sent over the revised {dequem} quote",
    "Still waiting on the {dequem} quote","The {dequem} quote was approved",
    "Noted, leave it with me","Send me the address when you can",
    "Just confirmed with {quem} by phone","{coisa} needs two signatures",
    "Found a better place, and closer too","{coisa} is late again, I called {quem}",
    "Everything went through with {coisa}","Managed to change the {dequem} schedule",
    "We'll need {n} more boxes","Paid the {dequem} invoice this morning"],
  quem=["the supplier","the courier","the accountant","Marina","the site crew","the building manager","the shop"],
  coisa=["The contract","The paperwork","The keys","The report","The invoice","The materials","The delivery",
         "The order","The renovation","The insurance","The move","The project"],
  acao=["bring forward","close","review","set aside","organise","double-check"],
  dia=["Monday","Tuesday","Wednesday","Thursday","Friday","the end of the month","next week"],
  dequem=["electrical","painting","freight","office","site"])

def long_chat(cfg, a, b, n, start, fmt, seed=5):
    r=random.Random(seed)
    def preenche(s):
        s=s.format(quem=r.choice(cfg["quem"]), coisa=r.choice(cfg["coisa"]), acao=r.choice(cfg["acao"]),
                   dia=r.choice(cfg["dia"]), dequem=r.choice(cfg["dequem"]), n=r.choice([2,3,5,10,15,20,30]))
        return s[0].upper()+s[1:]
    out=[]; t=start; i=0
    while len(out) < n:
        if r.random() < 0.35:
            for j,frase in enumerate(r.choice(cfg["pares"])):
                out.append(f"{t.strftime(fmt)} - {(a if (i+j)%2 else b)}: {preenche(frase)}")
                t += datetime.timedelta(minutes=r.randint(1,40))
            i+=2
        else:
            out.append(f"{t.strftime(fmt)} - {(a if i%2 else b)}: {preenche(r.choice(cfg['soltas']))}")
            t += datetime.timedelta(hours=r.choice([1,2,3,5,8,14,20,30]), minutes=r.randint(0,59))
            i+=1
    return out[:n]

def block(msgs, start, fmt, seed=11):
    r=random.Random(seed); out=[]; t=start
    for who,m in msgs:
        out.append(f"{t.strftime(fmt)} - {who}: {m}")
        t += datetime.timedelta(minutes=r.randint(2,12))
    return out
