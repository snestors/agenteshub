# Verification

> Cómo validar trabajo cuando `init.sh` no es suficiente. **template-managed** —
> los updates del template lo sobrescriben.

## Validación automatizada (init.sh)

`init.sh` cubre el camino dorado: tests + build + smoke según el stack
detectado. Eso es lo que el reviewer mira primero. Si init.sh falla, no hace
falta seguir.

## Validación manual cuando init.sh no alcanza

Hay clases de cambios que init.sh no cubre:

- **Cambios visuales / UI**: el reviewer (o vos) tiene que abrir la app y
  validar el comportamiento real. Anotar el navegador / dispositivo.
- **Cambios de performance**: medir antes y después. Reportar números reales,
  no "se siente más rápido".
- **Cambios de schema de DB**: correr la migración hacia adelante y hacia
  atrás en una DB temporal. Verificar que el `down` no rompa.
- **Integraciones externas**: si un webhook / API externa cambia, mockearla
  no alcanza. Validar contra el sandbox real.

## Checklist por tipo de cambio

### Backend handler nuevo

- [ ] `init.sh` verde
- [ ] Test que llama el handler con request válida y verifica response shape
- [ ] Test con input mal-formado verifica el código de error
- [ ] Si toca DB, test verifica tanto el insert como el read-back

### Frontend componente nuevo

- [ ] `init.sh` verde (build pasa)
- [ ] Validación visual en el navegador (golden path + 1 caso de error)
- [ ] El componente se desmonta limpio (sin warnings de listeners residuales)

### Migración de DB

- [ ] `init.sh` corre la migración en una DB temp
- [ ] Hay un test que verifica que el rollback (`down`) no pierde datos
- [ ] Si la migración es destructiva, está documentada en el commit message

### Cambio de contrato (API / env)

- [ ] `docs/architecture.md` actualizado en el mismo cambio
- [ ] Si rompe consumidores existentes, hay comm previa documentada

## Cuándo bypasear

Nunca. Si init.sh es insuficiente para una feature, agregar el chequeo a
`init.sh` o agregar instrucciones explícitas a este archivo. El reviewer
nunca aprueba con "no se chequeó porque era difícil".
