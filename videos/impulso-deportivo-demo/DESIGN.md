# Design System — Impulso Deportivo

## Overview

Impulso Deportivo es una plataforma SaaS para academias deportivas con identidad **editorial-deportiva**. La estética es de "newspaper sport" en mayúsculas condensadas — tipografía display Saira Condensed contrastando con cursivas en minúscula que rompen la rigidez. Layout denso pero ordenado, con superficies blancas, reglas finas y datos en fuente monoespaciada. Paleta acromática (blancos, grises, tinta) con un único acento teal profundo que aparece en CTAs, badges y stats. La sensación es de **producto serio para operadores reales**, no demo de startup.

## Colors

- **Ink Primary**: `#0F1213` — tinta casi negra para titulares y body
- **Ink Soft**: `#35393A` — tinta suavizada para subtítulos
- **Ink Muted**: `#6C6F71` — labels, hints, metadata
- **Ink Quiet**: `#A8ABAC` — placeholders, separadores blandos
- **Paper**: `#FFFFFF` — superficie principal
- **Paper Soft**: `#F9FAFB` — superficies secundarias (cards en lista)
- **Paper Muted**: `#F0F2F3` — bandas de filtros, headers de tabla
- **Rule**: `#DBDFE0` — reglas finas entre filas
- **Rule Strong**: `#BBBEBF` — bordes activos, separadores fuertes
- **Accent Teal**: `#225656` — color Impulso, CTAs primarios, badges activos
- **Accent Teal Bright**: `#03B2CB` — focus rings, links interactivos
- **Accent Teal Deep**: `#003131` — variante oscura para hover/active
- **Accent Soft**: `#E0F3F3` — fondos de chips activos, highlights
- **Success**: `#308639` — badges "Al día", "Pagada", "Confirmada"
- **Success Soft**: `#E7F6ED` — fondo de badges success
- **Warn**: `#9F7849` — badges "Vencida", "En mora"

## Typography

- **Display**: Saira Condensed (300–900). Titulares en MAYÚSCULAS condensadas, fontSize 92px en hero, 48px en sections, 32–40px en headers de pantalla. Trato característico: la palabra "academia" o "demo" se renderiza en minúscula cursiva dentro de un h1 mayúsculas → crea contraste editorial.
- **UI**: Space Grotesk (300–700). Body, labels, navegación, micro-copy. Tamaños 12–16px. Letter-spacing levemente abierto en labels (uppercase 0.05em).
- **Mono**: JetBrains Mono (400–600). Números (montos, fechas, IDs), badges con codes (`AL_DIA`, `BASICO`, `YAPE`), tabular nums activado (`font-feature-settings: "tnum" 1, "zero" 0`) para evitar slashed zero en stats.

## Elevation

Sistema **flat editorial** sin glassmorphism, sin shadows pesadas. La profundidad se construye con:
- **Reglas finas de 1px** color `#DBDFE0` entre filas y entre cards.
- **Cambio sutil de superficie** (`#FFFFFF` → `#F9FAFB`) para diferenciar áreas.
- **Border-radius bajo** (4–8px en cards, 6px en botones) — nunca pills excepto en chips de filtro.
- **Hover** sube la superficie un step (ej. paper-2 → paper) sin shadow — animación de 200ms ease.
- **Focus ring** de 2px en teal bright `#03B2CB` con 2px offset blanco.

## Components

- **Sidebar Editorial**: navegación lateral con logo "IMPULSO DEPORTIVO" en Saira Condensed (un poco más bajo que el título principal del topbar), items uppercase tracking ancho, item activo con fondo ink y texto paper.
- **Topbar Track**: barra superior con breadcrumb tipo `PLATAFORMA · SUPER ADMIN` en mono uppercase + nombre de página en display + avatar circular del user.
- **Stat Cards 4×1**: row de 4 cards con label uppercase mono pequeño + número gigante en mono (font-feature `tnum`). Reglas finas separan cards.
- **Tablas Editoriales**: thead uppercase tracking ancho fondo `#F0F2F3`, tbody filas con regla fina inferior, números right-aligned en mono.
- **Badges Tipados**: pills pequeñas con border 1px y bg soft según semántica (success-soft + ink-success para "Al día", warn-soft para "Vencida"). Texto uppercase mono.
- **Chips de Filtro**: row horizontal de pills, una activa con bg ink + text paper, resto bg paper-2 + text ink. Click = swap.
- **Botones**:
  - Primary: bg `#225656` ink white border-radius 6px padding 10×16, font display 14px uppercase tracking 0.05em.
  - Secondary: bg paper border 1px rule-strong text ink.
  - Ghost: bg transparent text ink hover bg paper-2.
- **Cards Lista**: card blanca border 1px rule, padding 16, h3 display + meta mono uppercase + tag chip a la derecha. Hover sube fondo a paper-2.
- **Form Fields Editoriales**: label uppercase mono pequeño + input border 1px rule, foco ring teal bright. Hint en mono `--font-mono` color ink-muted.
- **MediaViewerModal**: modal central con fondo backdrop ink/70, contenedor white max 90vw, preview de imagen o iframe PDF, footer con botón "Descargar".
- **Dot punto al final de títulos**: muchos h2 terminan con un punto (`DASHBOARD.`) marcando cierre editorial.

## Do's and Don'ts

### Do's

- Usar Saira Condensed MAYÚSCULAS para titulares y combinar con minúscula cursiva en una palabra clave para romper.
- Activar `font-feature-settings: "tnum" 1, "zero" 0` en todo número/badge para evitar slashed-zero.
- Mantener reglas finas 1px `#DBDFE0` como separador principal entre cards y filas.
- Usar el teal `#225656` solo para acentos verdaderos (CTAs, badges activos) — no decorar.
- Letter-spacing 0.05em en uppercase labels para airearlas.

### Don'ts

- No usar shadows pesadas — el sistema es flat con reglas finas.
- No usar pills/border-radius alto en cards o tablas — solo en chips de filtro y badges.
- No usar gradientes en superficies (el QA bug v0.23.1 ya quitó los del LoginPage).
- No mezclar más de 1 acento — solo teal Impulso. El verde success y el warn son semánticos, no decorativos.
- No usar fonts genéricas en lugar de Saira Condensed para titulares — la marca pierde su carácter.
- No usar el token fantasma `--ink-1` — usar `--ink` directo (regla del cleanup v0.27.1).
