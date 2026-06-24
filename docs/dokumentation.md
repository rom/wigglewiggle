# wigglewiggle — dokumentation

wigglewiggle är ett litet Windows-program som hindrar skärmen från att låsas
och datorn från att gå i viloläge så länge programmet körs. Det körs helt och
hållet med dina vanliga användarrättigheter och **kräver aldrig
administratörsbehörighet**.

> Obs: Programmets egna menytexter är på engelska (t.ex. *Mode*, *Interval*,
> *Quit*). I den här dokumentationen visas de engelska etiketterna precis som
> de står i programmet, med förklaringar på svenska.

## Innehåll

- [Vad programmet gör](#vad-programmet-gör)
- [De två metoderna](#de-två-metoderna)
- [Menyn i systemfältet](#menyn-i-systemfältet)
- [Starta vid inloggning](#starta-vid-inloggning)
- [Inga administratörsrättigheter krävs](#inga-administratörsrättigheter-krävs)
- [Bygga och köra](#bygga-och-köra)
- [Så är koden uppbyggd](#så-är-koden-uppbyggd)
- [Ikonen](#ikonen)
- [Tester](#tester)
- [Begränsningar och möjlig vidareutveckling](#begränsningar-och-möjlig-vidareutveckling)

## Vad programmet gör

När du startar wigglewiggle öppnas inget fönster. I stället visas en liten ikon
i **systemfältet** — samlingen med små ikoner längst ned till höger, bredvid
Windows-klockan. Klicka på ikonen för att öppna en meny där du väljer hur
programmet ska arbeta, pausar det tillfälligt eller avslutar det.

Själva arbetet med att hålla skärmen vaken kan göras på två olika sätt, och du
kan växla mellan dem när som helst via menyn.

## De två metoderna

### OS keep-awake — "OS-vakenhet" (standardläge)

I det här läget ber programmet helt enkelt Windows att hålla sig vaket. Inget
rör sig på skärmen — Windows nollställer bara sina inaktivitetstimrar, så att
datorn inte går i viloläge, släcker skärmen, startar skärmsläckaren eller visar
låsskärmen.

Eftersom ingenting flyttar på muspekaren eller skickar tangenttryckningar är
metoden helt osynlig. Den håller också datorn vaken även när du är borta från
den, vilket är själva poängen. Det här är samma teknik som verktyg av typen
PowerToys Awake använder.

### Input simulation — "Inmatningssimulering"

I det här läget låtsas programmet att du använder datorn. När du har varit
**inaktiv** under den valda tiden skickar det en liten gnutta påhittad
inmatning och växlar mellan två varianter:

1. en **F15-tangenttryckning** — F15 är en riktig tangent som så gott som inga
   program reagerar på, vilket gör den till den vanliga "låtsasaktiviteten"; och
2. en **liten musrörelse** — pekaren flyttas en bildpunkt åt höger och genast
   tillbaka, så att den hamnar exakt där den började.

Att växla mellan de två fångar program som bara håller koll på antingen
tangentbordet eller musen. Det här läget håller dessutom närvarostatusen i
chattprogram (Teams/Slack) "aktiv".

### Inaktivitetsmedvetenhet

Inmatningssimuleringen är **inaktivitetsmedveten**: den agerar bara när du
faktiskt har varit overksam under den valda tiden. Så länge du själv använder
datorn ligger den helt tyst och stör aldrig din riktiga mus eller ditt
tangentbord. Eftersom även den påhittade inmatningen nollställer Windows
inaktivitetstimer hamnar nästa knuff naturligt en hel intervalltid senare.

Inaktivitetsmedvetenheten gäller specifikt inmatningssimuleringen. OS-vakenhet
håller i stället en kontinuerlig "håll dig vaken"-begäran och styrs medvetet
inte av om du är inaktiv eller inte.

## Menyn i systemfältet

```
<statusrad>              t.ex. "Active — OS keep-awake"   (ej klickbar)
────────────
Mode ▸  ● OS keep-awake          (välj metod — fungerar som radioknappar)
        ○ Input simulation
Interval ▸  ○ 30 seconds         (gäller endast inmatningssimulering)
            ● 1 minute
            ○ 2 minutes
            ○ 5 minutes
────────────
Pause / Resume           (pausa respektive återuppta allt)
☑ Start at login         (starta automatiskt vid inloggning)
────────────
Quit                     (avsluta programmet)
```

| Menypost | Engelsk text | Vad den gör |
| --- | --- | --- |
| Status | *(t.ex.)* `Active — OS keep-awake` | Visar vad programmet gör just nu. Posten är gråtonad och går inte att klicka på. |
| Metod | `Mode ▸` | Väljer metod: `OS keep-awake` eller `Input simulation`. Endast en kan vara ikryssad. |
| Intervall | `Interval ▸` | Hur länge du måste vara inaktiv innan inmatningssimuleringen agerar: 30 sekunder, 1, 2 eller 5 minuter. Standard är 1 minut. |
| Paus | `Pause` / `Resume` | Stoppar tillfälligt all vakenhållning. Texten växlar mellan *Pause* och *Resume* beroende på läge. |
| Autostart | `Start at login` | Kryssruta som styr om programmet startar automatiskt när du loggar in. |
| Avsluta | `Quit` | Avslutar programmet och släpper "håll dig vaken"-begäran. |

Statusraden och texten som visas när du håller muspekaren över ikonen
uppdateras automatiskt varje gång du ändrar något.

## Starta vid inloggning

Om du kryssar i **Start at login** lägger programmet till en post i Windows så
att wigglewiggle startar automatiskt varje gång du loggar in. Avbockning tar
bort posten igen.

Tekniskt sker detta i Windows-registret, under den **per-användare**-lista som
heter `Run` (`HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Run`).
Den listan tillhör ditt eget konto och kan ändras utan administratörsrättigheter.
Posten pekar på programmets egen plats på disken, omsluten av citattecken så att
en sökväg med mellanslag (t.ex. under `Program Files`) tolkas rätt.

## Inga administratörsrättigheter krävs

Allt programmet gör är åtgärder på din egen användarnivå. Inget av det kräver
förhöjda rättigheter:

- **`SetThreadExecutionState`** sätter en "håll dig vaken"-begäran för den
  anropande tråden i det egna programmet.
- **`SendInput`** och **`GetLastInputInfo`** arbetar inom din inloggade
  skrivbordssession.
- **Autostarten** skriver till `HKEY_CURRENT_USER` — den datoromfattande
  motsvarigheten (`HKLM`) skulle kräva administratör, men den undviker vi
  medvetet.

## Bygga och köra

Kräver Go 1.23 eller senare. Bygget blir en enda körbar fil utan konsolfönster:

```sh
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -ldflags "-H=windowsgui -s -w" -o wigglewiggle.exe .
```

`GOARCH=arm64` fungerar på samma sätt. Bygget är ren Go (`CGO_ENABLED=0`), så
ingen C-verktygskedja behövs.

Starta sedan `wigglewiggle.exe` — ikonen dyker upp i systemfältet. Klicka för
menyn och välj **Quit** för att avsluta.

## Så är koden uppbyggd

```
main.go                                    ikon och meny i systemfältet samt programmets livscykel
internal/keepawake/engine.go               själva logiken: en liten "motor" som körs i bakgrunden
internal/win/win_windows.go                tunna omslag runt de Windows-funktioner som används
internal/autostart/autostart_windows.go    posten i registret för "starta vid inloggning"
internal/ui/icon.go + icon.ico             den inbäddade ikonbilden
tools/genicon/                             ett hjälpprogram som ritar och sparar ikonen
docs/                                       dokumentation (design.md samt denna fil)
```

Hela programmet är endast avsett för Windows, så källkodsfilerna byggs för
Windows (antingen via filnamn som slutar på `_windows.go` eller via en
`//go:build windows`-rad) och korskompileras med `GOOS=windows`. Ikonbilden och
hjälpprogrammet som ritar den är plattformsoberoende.

**Hur delarna hänger ihop:** `main.go` bygger menyn och äger inga svåra beslut
själv — när du klickar på något säger den till motorn i `keepawake` vad som ska
hända. Motorn använder i sin tur `win`-paketet för att prata med Windows, och
`autostart` för registerinställningen.

### Trådmodellen (lite mer tekniskt)

Windows kommer ihåg "håll dig vaken"-begäran per **tråd** (en enskild
exekveringslinje) och glömmer den om den tråden avslutas. Normalt får Go flytta
kod mellan trådar, vilket skulle kunna släppa begäran av misstag. Därför kör
motorn en enda bakgrundsuppgift som låser sig vid en tråd (`LockOSThread`) under
hela sin livstid och utför alla Windows-anrop därifrån. Uppgiften vaknar var
femte sekund, samt direkt när du ändrar en inställning, och ser till att
verkligheten stämmer med inställningarna. När programmet avslutas släpps
begäran så att vanlig vilo-/låsfunktion återgår.

## Ikonen

Hjälpprogrammet `tools/genicon` ritar ett vänligt turkost "vaket" ansikte i
storlekarna 16, 32, 48 och 64 bildpunkter och sparar dem i en `.ico`-fil.
Bilden bäddas in i programmet, så den färdiga `.exe`-filen bär sin egen ikon
och behöver ingen separat bildfil. Vill du ändra utseendet kör du:

```sh
go run ./tools/genicon
```

## Tester

- `internal/ui` har ett test som körs på vilken dator som helst och
  kontrollerar att den inbäddade `.ico`-filen är giltig
  (`go test ./internal/ui/`).
- `internal/win` har ett Windows-test som vaktar minneslayouten för den
  `INPUT`-struktur som `SendInput` förlitar sig på (kontrolleras via
  `GOOS=windows go vet ./...` och körs i Windows-miljö).

Själva Windows-beteendet går inte att provköra på en dator som inte kör
Windows; det säkerställs genom korskompilering och `go vet` och är tänkt att
snabbtestas på en riktig Windows-dator.

## Begränsningar och möjlig vidareutveckling

- I praktiken endast 64-bitars (`INPUT`-layouten förutsätter 8-bytes pekare;
  moderna Windows-mål är amd64/arm64).
- Inga sparade inställningar utöver "starta vid inloggning" — vald metod och
  intervall återgår till standard vid varje start. En liten inställningsfil
  skulle kunna läggas till vid behov.
- Inga avisering-bubblor; tillståndet visas via verktygstipset och den
  gråtonade statusraden.

---

Se även den engelska designbeskrivningen i [design.md](design.md).
