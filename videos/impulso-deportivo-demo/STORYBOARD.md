# Storyboard — Impulso Deportivo Product Demo

**Format:** 1920×1080
**Duration target:** ~52 seconds
**Audio:** edge-tts voiceover (Spanish PE) + minimal underscore + selective SFX
**VO direction:** voz femenina o masculina neutral peruana, calma y confiada — registro tipo "presentación de producto serio". Pausas naturales entre frases. La frase final cae con peso, no apurada.
**Style basis:** DESIGN.md — paleta blanco / ink / teal Impulso, tipografía Saira Condensed display, datos en JetBrains Mono, sin shadows pesadas, reglas finas como separadores, estética editorial-deportiva.

**Underscore:** mínimo. Pad cálido sostenido en teal-mood (think: minimal electronic, casi imperceptible). Sube ligeramente en Beat 6 al CTA. Nunca compite con la VO.

**Global guardrails específicos:**
- La paleta es light. NO oscurecer escenas — el blanco editorial es la marca. Acentos en teal `#225656` y `#03B2CB`.
- Motion deliberado pero contenido. Editorial > flashy. Cada movimiento tiene una razón narrativa.
- Datos en mono con `font-feature-settings: "tnum" 1, "zero" 0` — el slashed-zero rompe la marca.
- El logo "IMPULSO DEPORTIVO" aparece en Beat 1 y Beat 6.
- Reglas finas (1px `#DBDFE0`) como separador estético recurrente — heredan del UI real.

---

## Asset Audit

| Asset | Type | Assign to Beat | Role |
|---|---|---|---|
| `capture/assets/impulso-logo-black.png` | Logo PNG | Beat 1, Beat 6 | Brand mark opener + closer |
| `capture/screenshots/scroll-000.png` | Landing hero | Beat 1 (BG fade) | Background fade-in del hook |
| `capture/screenshots/auth/admin-dashboard.png` | UI screenshot | Beat 2 | Dashboard super admin con stats reales — Ken Burns |
| `capture/screenshots/auth/panel-academia-dashboard.png` | UI screenshot | Beat 3 | "Empezá por acá" del owner |
| `capture/screenshots/auth/panel-profesores-invitar-modal.png` | UI screenshot | Beat 3 | Modal invitar profe |
| `capture/screenshots/auth/panel-grupo-detail.png` | UI screenshot | Beat 3 | Detalle grupo con horarios |
| `capture/screenshots/auth/panel-academia-clases.png` | UI screenshot | Beat 4 | Vista global clases (alterna con asistencia) |
| `capture/screenshots/auth/panel-alumno-detail.png` | UI screenshot | Beat 5 | Estado de cuenta — apoderado side |
| `capture/screenshots/auth/panel-alumno-detail-cobros.png` | UI screenshot | Beat 5 | Cobros con comprobante |
| `capture/screenshots/auth/admin-academias-list.png` | UI screenshot | Beat 2 (cameo) | Floating card lateral |
| `capture/screenshots/auth/panel-academia-alumnos.png` | UI screenshot | Beat 3 (cameo) | Floating card lateral |
| `capture/screenshots/auth/panel-grupos-list.png` | UI screenshot | Beat 3 | Cameo de cards en cascada |
| `capture/screenshots/auth/login-design.png` | UI screenshot | SKIP (Beat 1 si hace falta) | Reserva |
| `capture/screenshots/scroll-035.png` `scroll-069.png` `scroll-100.png` | Landing scrolls | SKIP | Material crudo, no aporta al producto |

**Cobertura de assets**: 9 de 13 screenshots auth + logo + 1 landing = 11 assets activos. Cumple el 50% de utilización mínima.

---

## BEAT 1 — HOOK (0.00s – 4.14s · 4.14s)

**VO:** "Una academia deportiva no se gestiona en un cuaderno."

**Concept:** Apertura tipográfica editorial. Sin imagen de stock. Sin demo prematuro. La frase del hook llega como un titular de portada de revista deportiva — Saira Condensed mayúsculas + minúscula cursiva subrayando "cuaderno" para crear el contraste irónico. El cuaderno es el dolor; lo que viene después es la solución.

**Visual:**
- Fondo blanco puro `#FFFFFF`. Una sola regla horizontal fina `#DBDFE0` arriba a 80px del top, otra abajo a 80px del bottom — encuadre tipo periódico.
- En la regla superior, mono uppercase a la izquierda: `IMPULSO · DEPORTIVO · DEMO` (12px tracking 0.15em color ink-3). A la derecha: `2026.05`.
- Centro: la frase en 4 líneas, Saira Condensed 800 weight, 84px line-height 0.95 color ink:
  - "UNA"
  - "ACADEMIA DEPORTIVA"
  - "NO SE GESTIONA EN UN"
  - *"cuaderno."* — esta línea en Saira Condensed 600 italic minúsculas color ink-2 (mismo tamaño 84px).
- Las primeras 3 líneas TYPE ON letra por letra (cursor mono blink), 60ms por char, total ~2.2s.
- La palabra "cuaderno." aparece después con un FADE + scale 0.96 → 1.0 con leve rotación -1° → 0°, 0.6s power2.out. Subraya con una línea SVG dibujada de izq a der, 0.5s ease.

**Mood direction:** Apertura tipo editorial New York Times sport section. Confianza, no decoración. Silencio antes de hablar — "te voy a decir algo".

**Assets:** `capture/assets/impulso-logo-black.png` aparece arriba a la derecha al final del beat (último 0.5s) en escala mini, 24px de alto, con fade-in sutil — anticipo del Beat 6.

**Animation choreography:**
- Reglas top/bottom: DRAW de 0% a 100% width, 0.4s power2.out, escalonadas 0.1s.
- Mono uppercase header: types on en paralelo a las reglas, 0.4s.
- Líneas 1–3 del hook: type on cascade, cursor mono blink, ~2.2s total.
- Línea "cuaderno." (italic): fade-in + subtle rotate, 0.6s.
- Underline SVG bajo "cuaderno": draw L→R, 0.5s ease.
- Logo Impulso top-right: fade-in 0.4s al final.

**Transition OUT:** Velocity-matched downward — exit y:+120 blur:18px 0.35s power2.in. La energía es "página que avanza".

**Depth layers:** BG: paper. MG: tipografía + reglas. FG: logo Impulso (cameo último 0.5s).

**SFX:** Click sutil tipográfico en cada line break (3 ticks suaves). En "cuaderno." un click de pluma metálica analógica. Pad de fondo entra en silencio absoluto, sube a -28dB en 1s.

---

## BEAT 2 — LA PROMESA (4.14s – 10.76s · 6.62s)

**VO:** "Impulso Deportivo es la plataforma que reúne todo en un solo lugar: alumnos, asistencia, pagos y reportes."

**Concept:** Pasamos del titular al producto. El dashboard del super admin se desliza desde abajo como evidencia. Counters animados en los stat cards. La pantalla NO se queda quieta — la recorremos en Ken Burns sutil mientras counters cuentan hacia los números reales.

**Visual:**
- Fondo paper. Cuatro stat cards FLOATING al frente con bg paper, border 1px rule, sin la pantalla completa.
- En realidad, montamos una **réplica fiel** de la sección stats del dashboard (mejor calidad que screenshot crudo):
  - 4 cards row 4×1: `TOTAL ACADEMIAS / ACTIVAS / SUSPENDIDAS / TOTAL USUARIOS`
  - Labels uppercase mono 11px tracking 0.1em ink-3
  - Números: JetBrains Mono 700 weight, 92px, ink, `font-feature: tnum, zero off`
  - Counters animados: 0 → 5, 0 → 5, 0 → 0, 0 → 54, todos 1.2s power2.out con stagger 0.15s
- Detrás (background midground), screenshot **`admin-dashboard.png`** desenfocado 8px blur con leve Ken Burns scale 1.02 → 1.05 sobre 8s.
- A la derecha del frame, dos UI cards "FLOTANTES" con menor opacidad (0.45):
  - `admin-academias-list.png` mini, rotada -3°, top-right
  - `panel-academia-alumnos.png` mini, rotada +2°, bottom-right
  - Slow drift vertical loop ±8px, 4s.
- Sobre las stat cards, palabras clave appear sincronizadas con la VO en un row de chips abajo:
  - `ALUMNOS` `ASISTENCIA` `PAGOS` `REPORTES`
  - Cada chip aparece cuando se nombra: bg accent-soft `#E0F3F3` border 1px teal `#225656` text teal padding 6×12 border-radius 999.
  - Stagger 0.5s entre chips.

**Mood direction:** Producto serio en evidencia. La cámara observa — no presume. Estilo "operador mira su dashboard de mañana".

**Assets:** `admin-dashboard.png` (BG), `admin-academias-list.png`, `panel-academia-alumnos.png` (cameos laterales).

**Animation choreography:**
- Slide-up de las 4 stat cards, stagger 0.12s desde y:+80 blur:12 → 0, 0.6s power3.out.
- Counters COUNT UP en cada card, 0 → final, 1.2s power2.out (números mono — el `tnum` es crítico).
- BG dashboard: Ken Burns slow zoom 1.02 → 1.05, 8s linear.
- Cameos laterales: float-in fade desde derecha y leve rotate, drift loop continuo.
- Chips: pop-in stagger sincro con VO, scale 0.85 → 1.0 + opacity, 0.3s back.out.

**Transition OUT:** Cross-Warp Morph (shader transition, 0.6s power2.inOut) — esta es la entrada al "demo product feel" del Beat 3.

**Depth layers:** BG: dashboard.png blurred + Ken Burns. MG: stat cards + counters. FG: chips de keywords.

**SFX:** Soft "tick" mecánico cada vez que un counter pasa un dígito (sutil, casi imperceptible — 6dB bajo VO). Pop suave en cada chip.

---

## BEAT 3 — EL DUEÑO ARMA SU ACADEMIA (10.76s – 16.06s · 5.30s)

**VO:** "Desde el panel del dueño, invitás a tus profes y armás tus grupos con horarios reales."

**Concept:** Doble foco. Primero el modal de invitar profe aparece como evidencia sintética (real screenshot reencuadrado), luego transición al detalle de un grupo con horarios visibles. El viewer ve los dos momentos clave del onboarding del owner: invitar gente + estructurar grupos.

**Visual — Sub-beat 3a (0:13–0:18):**
- Fondo paper-2 `#F9FAFB`.
- Centro: card grande con `panel-profesores-invitar-modal.png` reencuadrado al modal solamente (crop centrado al diálogo). Border 1px rule, border-radius 8px, scale 0.95 entrada.
- A los lados, cards mini cascading:
  - Izq: `panel-academia-dashboard.png` mini bg paper, rotate -4°, opacity 0.5
  - Der: `panel-academia-alumnos.png` mini bg paper, rotate +3°, opacity 0.5
- Sobre el modal central, 3 chips uppercase mono aparecen sincronizados:
  - `IDENTIFICADOR` `NOMBRE` `LINK GENERADO`
  - Cada uno highlight el campo del modal con un overlay teal-soft transparente (pulse 0.4s).

**Visual — Sub-beat 3b (0:18–0:23):**
- Cross-cut a una vista "schedule grid" sintética + el screenshot `panel-grupo-detail.png` cropped a la sección horarios.
- Panel grupo a la izquierda (ancho 60%), schedule grid sintética a la derecha (ancho 40%):
  - Grid de 7 columnas (Lun..Dom) × 12 filas (8:00..20:00) con cells small.
  - 4 cells highlighted en teal `#225656` con label mono "VOLEY M-15" — los horarios del grupo.
  - Cells aparecen con un STAMP (scale 0.7 → 1.0 + opacity, stagger 80ms entre cells, total 0.5s).

**Mood direction:** "El owner construye". Estética constructiva. Cada elemento es un bloque que se coloca con intención. Editorial, no flashy.

**Assets:** `panel-profesores-invitar-modal.png` (centro 3a), `panel-academia-dashboard.png` `panel-academia-alumnos.png` (cameos), `panel-grupo-detail.png` (centro 3b).

**Animation choreography:**
- 3a → modal central: zoom-in 0.95 → 1.0 + fade, 0.5s power2.out.
- 3a → cards laterales: drift fade + rotate, 0.7s power2.out, stagger 0.15s.
- 3a → chips de campos: cascade aparición + pulse highlight sobre el modal, stagger 0.25s.
- Transición 3a → 3b: hard cut con whip-pan blur 24px 0.25s.
- 3b → panel grupo: slide desde izq (x:-200 → 0), 0.5s power3.out.
- 3b → schedule grid cells: STAMP cascade, 0.5s total.

**Transition OUT:** Velocity-matched upward — exit y:-120 blur:24px 0.35s power2.in.

**Depth layers:** BG: paper-2 fill. MG: cards principales. FG: chips highlight + grid cells.

**SFX:** En 3a, soft "click" cuando aparece el LINK GENERADO chip. En 3b, ligero "tap" cada cell del grid (rítmico, casi imperceptible — patrón de 4 ticks).

---

## BEAT 4 — EL PROFESOR EN LA CANCHA (16.06s – 20.91s · 4.85s)

**VO:** "Tus profes toman asistencia desde el celular y queda registrado al instante."

**Concept:** Cambio de superficie — el viewer entiende que esto pasa ON-LOCATION, no en oficina. Mostramos el panel del profesor ESTILIZADO como vista mobile/tablet. Una lista de alumnos donde los radio buttons "Presente" se van marcando uno por uno con cascade. El "queda registrado al instante" lo demuestra el side-effect de la clase pasando a `realizada` con un check verde grande.

**Visual:**
- Fondo paper. Centro: **mockup mobile** estilizado (frame iPhone-ish, ratio ~9:19, ancho 380px) con la pantalla de asistencia.
- Dentro del mockup:
  - Header sticky: "ASISTENCIA · 19:00 · VÓLEY M-15" en display 18px ink + meta mono.
  - 5 filas de alumnos (cada fila 56px alto):
    - Avatar inicial (círculo 32px teal `#225656` con letra paper).
    - Nombre Saira Condensed 600 16px.
    - 4 botones radio horizontales: `PRESENTE` `TARDANZA` `JUSTIF.` `INJUSTIF.` (chips small).
  - Las filas aparecen stagger desde abajo, 0.3s cada una.
  - Una vez todas en pantalla, el botón `PRESENTE` se ACTIVA (bg teal, text paper) en cascade fila por fila, 200ms entre filas. Total ~1s.
- Al fondo del mockup, un chip pequeño en topbar `CLASE: PROGRAMADA` pasa a `REALIZADA` con un crossfade y el color cambia a success-soft `#E7F6ED` con text success `#308639`.
- Detrás del mockup mobile, texto grande tipográfico (background midground): "EN VIVO." en Saira Condensed 800 italic 180px color paper-2 (apenas visible — atmósfera, no protagonista).

**Mood direction:** Movimiento real. La cancha. Una decisión por click. El operador sintiendo que el sistema responde.

**Assets:** Mockup sintético (no screenshot directo — más limpio). Optional cameo de `panel-academia-clases.png` esquina inferior izquierda con opacity 0.25 como contexto.

**Animation choreography:**
- Mockup mobile: zoom-in scale 0.85 → 1.0 + fade, 0.5s power2.out.
- Filas alumnos: stagger slide-up desde y:+30, 0.3s power2.out cada una, total 1.5s.
- Botón PRESENTE activate cascade: bg cambia teal con scale-bump 1.0 → 1.05 → 1.0 + paper-text fade-in, stagger 200ms entre filas.
- Chip "PROGRAMADA → REALIZADA": crossfade 0.4s al final.
- Texto BG "EN VIVO.": fade-in lento 0.8s → opacity final 0.08, drift sutil ±10px.

**Transition OUT:** Cross-Warp Morph 0.6s — entramos al mundo del apoderado.

**Depth layers:** BG: paper + texto "EN VIVO." atmosférico + cameo lateral. MG: mockup mobile. FG: las filas + check de "REALIZADA".

**SFX:** Tap mobile-feel suave en cada activación de PRESENTE (rítmico). Un "✓" sonoro positivo en el cambio "PROGRAMADA → REALIZADA".

---

## BEAT 5 — APODERADO PAGA, OWNER CONFIRMA (20.91s – 27.92s · 7.01s)

**VO:** "Los apoderados ven el estado de cuenta de sus hijos, pagan con foto del comprobante, y vos confirmás de un click."

**Concept:** La gran cadena de cobranza visualizada como un flujo de tres pantallas. Split screen narrativo: a la izquierda el panel apoderado con su estado de cuenta + el modal de pago apareciendo, a la derecha el panel del owner con el cobro `pendiente_confirmacion` que se confirma. La transición no es entre escenas, es DENTRO del mismo beat — el viewer ve el flujo completo.

**Visual:**
- Beat dividido en 3 micro-momentos sincronizados con la VO:
  - **5a (0:30–0:33)** "ven el estado de cuenta": split izq con `panel-alumno-detail.png` cropped a la EstadoCuentaCard. Fade-in + slide desde izq, 0.5s.
  - **5b (0:33–0:37)** "pagan con foto del comprobante": modal de pago aparece encima, escalado 0.9 → 1.0. Dentro del modal, simulamos un thumbnail de comprobante (rectángulo 80×100 con icon de imagen + meta "yape-screenshot.jpg" mono). Highlight pulse teal en el thumb.
  - **5c (0:37–0:42)** "vos confirmás de un click": split-right entra con `panel-alumno-detail-cobros.png` cropped a la tabla de cobros. Una fila con badge `PENDIENTE_CONFIRMACION` (warn-soft) hace HIGHLIGHT pulse, después un cursor sintético clickea un botón `CONFIRMAR`, y el badge cambia a `CONFIRMADO` (success-soft con check). En paralelo, otra fila del mismo grid: la mensualidad asociada cambia su badge `PENDIENTE` → `PAGADA` (success). Dual update sincronizado con un STAMP.
- Background: paper-2 fill con una regla vertical fina al centro `#DBDFE0` separando los dos lados.
- Top-bar editorial pequeño: izq mono `APODERADO` der mono `OWNER` — uppercase tracking, ink-3, 11px.

**Mood direction:** Conexión causa-efecto. Lo que el apoderado hace, el owner lo ve. La transparencia es el producto.

**Assets:** `panel-alumno-detail.png` (cropped EstadoCuentaCard), `panel-alumno-detail-cobros.png` (cropped tabla cobros).

**Animation choreography:**
- 5a izq: slide-from-left + fade, 0.5s power3.out.
- 5b modal: scale-in 0.9 → 1.0 + soft drop-shadow appear, 0.4s back.out. Thumb comprobante: pulse 1.0 → 1.05 → 1.0 ring teal, 0.6s loop 2 veces.
- 5c der: slide-from-right + fade, 0.5s power3.out, paralelo a la pulse del thumb.
- 5c cursor sintético: aparece y mueve sobre el botón `CONFIRMAR`, 0.5s lineal. Click pulse 0.2s.
- 5c badge change: crossfade `PENDIENTE_CONFIRMACION` → `CONFIRMADO`, 0.3s. STAMP scale 1.0 → 1.1 → 1.0 + glow brief teal.
- 5c badge mensualidad: paralelo crossfade `PENDIENTE` → `PAGADA`, mismo timing.

**Transition OUT:** Velocity-matched upward — exit y:-150 blur:30px 0.4s power2.in. La energía sube hacia el cierre.

**Depth layers:** BG: paper-2 + regla central. MG: pantallas split izq/der. FG: modal + cursor + badges en cambio.

**SFX:** Soft "ka-chunk" cuando el modal aparece en 5b. Click distintivo cuando el cursor sintético pulsa CONFIRMAR. Subtle "✓" success al cambio dual de badges.

---

## BEAT 6 — CIERRE (27.92s – 42.50s · 14.58s)

**VO:** "Sin Excel sueltos. Sin capturas perdidas en WhatsApp. Toda tu academia, en un solo panel. Impulso Deportivo. La plataforma para academias que quieren dejar de improvisar."

**Concept:** Cierre editorial de página de revista. Logo Impulso Deportivo grande. Tagline. URL. Una sola regla horizontal abajo. Rest. La energía de los beats anteriores se decanta acá.

**Visual:**
- Fondo paper. Reglas horizontales arriba y abajo (igual encuadre que Beat 1 — cierre simétrico al hook).
- Centro vertical:
  - Logo `impulso-logo-black.png` escalado 280px ancho. Aparece con FADE + scale 0.95 → 1.0, 0.6s power2.out.
  - Debajo, dos líneas de tagline en Saira Condensed 700 38px ink:
    - "TODA TU ACADEMIA,"
    - "EN UN SOLO panel."
    - La palabra "panel." en italic 600 ink-2 — cierre estético consistente con el hook.
  - Debajo de la tagline, mono uppercase 14px tracking 0.15em ink-3: `IMPULSODEPORTIVO.COM` (placeholder URL).
- Antes que la tagline, en el sub-beat de "Sin Excel sueltos. Sin capturas perdidas en WhatsApp" (0:42–0:46), aparece un strikethrough animation:
  - 2 chips arriba sobre la línea superior: `[EXCEL SUELTOS]` `[CAPTURAS WHATSAPP]` color ink en mono.
  - Una línea SVG diagonal-strike DRAW sobre cada chip (rojo `#9F4230` o ink-2 sobrio), 0.4s cada una stagger 0.3s.
  - Después de la VO "en un solo panel", los chips fade-out (0.4s) y aparece el logo + tagline.

**Mood direction:** Resolución. Calma. Confianza definitiva. "Listo. Ya está." El beat tiene tiempo para respirar — no apurés.

**Assets:** `capture/assets/impulso-logo-black.png`.

**Animation choreography:**
- Reglas top/bottom: DRAW 0% → 100% width, 0.4s power2.out al inicio del beat.
- Chips "EXCEL SUELTOS" / "CAPTURAS WHATSAPP": fade-in stagger 0.3s al principio.
- Strike-through SVG sobre cada chip: DRAW diagonal, 0.4s stagger 0.3s.
- Chips fade-out: 0.4s easeIn al final del primer sub-beat (~0:46).
- Logo: scale 0.95 → 1.0 + fade, 0.7s power2.out a partir de 0:46.
- Tagline lines: stagger fade + slide-up 16px, 0.4s cada una, 0.3s gap.
- URL mono: fade-in 0.4s al final.
- Hold final: 1.5s estática antes del end-frame.

**Transition OUT:** Hard cut a paper limpio (end frame).

**Depth layers:** BG: paper + reglas. MG: chips strikethrough → logo + tagline. FG: URL mono.

**SFX:** Pluma-strike sound cada vez que un chip se tacha. Pad de fondo sube ligeramente al aparecer el logo (+3dB). Resuelve en una nota cálida sostenida en los últimos 1.5s. Fade out final.

---

## Production Architecture

```
impulso-deportivo-demo/
├── index.html                    root — VO + underscore + beat orchestration
├── DESIGN.md                     brand reference
├── SCRIPT.md                     narration text
├── STORYBOARD.md                 THIS FILE — creative north star
├── transcript.json               word-level timestamps (Step 5)
├── narration.wav                 TTS audio (Step 5)
├── capture/                      captured website data
│   ├── screenshots/
│   │   ├── scroll-000.png ... scroll-100.png
│   │   └── auth/
│   │       ├── admin-dashboard.png
│   │       ├── admin-academias-list.png
│   │       ├── admin-academia-detail.png
│   │       ├── login-design.png
│   │       ├── panel-academia-dashboard.png
│   │       ├── panel-academia-alumnos.png
│   │       ├── panel-academia-grupos.png
│   │       ├── panel-academia-clases.png
│   │       ├── panel-alumno-detail.png
│   │       ├── panel-alumno-detail-cobros.png
│   │       ├── panel-grupo-detail.png
│   │       ├── panel-profesores-list.png
│   │       └── panel-profesores-invitar-modal.png
│   ├── assets/
│   │   ├── impulso-logo-black.png
│   │   └── svgs/
│   ├── extracted/
│   │   ├── tokens.json
│   │   ├── visible-text.txt
│   │   └── asset-descriptions.md
│   ├── AGENTS.md
│   └── CLAUDE.md
└── compositions/
    ├── beat-1-hook.html
    ├── beat-2-promesa.html
    ├── beat-3-armado.html
    ├── beat-4-asistencia.html
    ├── beat-5-cobranza.html
    ├── beat-6-cierre.html
    └── captions.html
```
