---
name: vault-secrets
description: Cómo leer y usar credenciales encriptadas del vault sin
  filtrarlas. Reglas duras de manejo y patrones de uso típicos.
---

# Vault de credenciales — guía dura

El vault guarda tokens, API keys y passwords cifrados con AES-GCM. La
master key vive en `cfg.SecretKey` del daemon. **Vos nunca la ves.**

## Tools disponibles

- `secret_list` — devuelve solo metadatos (key, description, scope,
  expires_at). **Nunca** el valor. Usalo para descubrir qué hay.
- `secret_get(key)` — devuelve el plaintext del secret. Solo invocalo
  cuando lo vas a usar inmediatamente. Cada lectura actualiza
  `last_accessed_at` (auditoría).

## Reglas DURAS — sin excepciones

1. **NO ECHO**. Nunca repitas el valor del secret en tu respuesta al
   user. Si te pregunta "¿cuál es el token?", respondé que está
   guardado y mostrá solo la `key`, nunca el value.
2. **NO LOG**. No escribas el value en `record`, `update_topic_state`,
   ni en cuerpos de mensaje. El plaintext debe vivir solo en la memoria
   del turno actual.
3. **NO COPY-PASTE A OTRO TOPIC**. Si tu turno necesita el secret y
   delegás con `run_in_topic`, no se lo pases en el prompt — la otra
   session llama `secret_get` por su cuenta.
4. **CUIDADO CON BASH**. Si lo metés en un comando, asegurate de que
   no quede en `journalctl`/history. Preferí variables de entorno
   inline:
   ```
   API_KEY="$(secret)" curl -H "Authorization: Bearer $API_KEY" ...
   ```
   en lugar de pasarlo como argumento literal.
5. **NO ABRIR EN PANTALLA SIN MOTIVO**. Si el user te pide "mostrame el
   token", confirmá que tiene acceso al `/vault` UI y dejá que lo
   revele desde ahí (el reveal masked + copy en la UI ya está
   pensado para eso).

## Patrones de uso típico

### Llamada HTTP autenticada

```
1. secret_get("CLOUDFLARE_TOKEN")  → te devuelve el plaintext
2. WebFetch / Bash con curl usando el token en el header
3. Olvidate del valor inmediatamente. No lo repitas en el resultado.
```

### Decisión condicional

```
1. secret_list  → ver qué keys existen
2. Si la que necesitás no está, decile al user qué falta y cómo
   crearla (UI: /vault → '+ nuevo'), no inventes una.
```

### Mini-agente que rota tokens

```
agent_create({
  name: "token-watcher",
  engine: "ollama",      # tarea liviana, sin claude
  system_prompt: "Sos un watchdog. Llamá secret_list, identificá los
                  que vencen en <7 días, devolvé JSON {expiring: [keys]}.
                  No leas valores. Sin echo."
})
```

## Convenciones de naming

- **Mayúsculas con underscores**: `BBVA_API_KEY`, `CLOUDFLARE_R2_KEY`,
  `SMTP_PASSWORD`. Mismo estilo que vars de entorno.
- **Prefijo por servicio**: `<SERVICE>_<TIPO>` para agruparlas mentalmente.
- **Scope** cuando aplique: `project:<id>` o `agent:<name>` para vincular
  la credencial a una unidad. `global` cuando es transversal.

## Qué NO hace el vault

- No rota automáticamente. Vos (el user) o un mini-agente con código
  custom rotan.
- No alerta de expiración por sí solo (eso lo decide un cron de un
  mini-agente).
- No tiene jerarquía / scopes herradados. `scope` es solo metadatos
  para filtrar mentalmente.

## Si el secret no existe

`secret_get(key)` devuelve error `"not found: <key>"`. **No falles
silenciosamente** ni inventes un value. Decile claro al user que el
secret no está cargado y cómo agregarlo (UI o `POST /api/secrets`).

## Recordá la jerarquía

```
user                  →  ve plaintext (UI reveal con confirm)
main agent            →  ve plaintext (vía secret_get)
mini-agent / topic    →  ve plaintext (vía secret_get)
session_messages DB   →  NUNCA debe contener plaintext
records               →  NUNCA
topic_state           →  NUNCA
mensaje al user       →  NUNCA
```

Si por error escribís un value en alguno de los "NUNCA", marcalo como
incidente: `record({topic:"vault-incidents", data:"<sin el value>"})`
y avisá al user para que rote la key inmediatamente.
