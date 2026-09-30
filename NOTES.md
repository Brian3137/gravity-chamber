# Notas

Tres líneas por cápsula: qué aprendí, qué me sorprendió, qué sigo sin entender.
La línea de "me sorprendió" es la valiosa — la sorpresa marca dónde estaba mal mi
modelo mental.

## Cápsula 00 — Kame House

- Me sorprendió:
- Pregunta abierta:

## Cápsula 01 — Construir el Scouter

- Aprendí: que healthz verifica que el proceso esté vivo y readyz verifica si está listo para recibir tráfico.
- Me sorprendió: Shutdown() no es simplemente "apagar el servidor"; primero deja de recibir tráfico nuevo y permite terminar los requests que ya estaban en curso, mientras que Close() puede cortar esos requests.
- Pregunta abierta: ¿cómo hace realmente un Load Balancer para detectar readyz y sacar temporalmente un servidor del pool?

## Cápsula 02 — Containers

- Aprendí: que un container no es una máquina virtual; empaquetamos Scouter en una imagen Docker usando un multi-stage build, una imagen distroless y un usuario non-root.
- Me sorprendió: una imagen ingenua de 1.37 GB terminó en 14.8 MB porque para ejecutar Scouter no necesitamos el compilador de Go ni todo el entorno de construcción.
- Pregunta abierta: ¿cómo se distribuye una imagen Docker a otras máquinas y cómo se ejecutan varias réplicas detrás de un Load Balancer?

## Cápsula 03

### Aprendí
Terraform es declarativo y convergente; `plan` muestra cambios y `apply` los ejecuta. Docker Compose permite levantar y conectar servicios usando nombres. El state registra lo que Terraform administra y no debe subirse a Git.

### Me sorprendió
Que Terraform detecte drift y que un cambio aparentemente simple pueda provocar un `destroy + create`. También que los containers se comuniquen por nombre dentro de la red de Compose.

### Pregunta
¿Cómo funciona el state de Terraform y cómo se conecta todo esto con GCP y Cloud Run?


| --cpus | tiempo | speedup vs 0.25 | lineal sería |
|--------|--------|------------------|--------------|
| 0.25   | 117.11s | 1.00×           | 1×           |
| 0.5    | 65.66s  | 1.78×           | 2×           |
| 1      | 38.285s | 3.06×           | 4×           |
| 2      | 6.140s  | 19.07×          | 8×           |
| 4      | 3.235s  | 36.20×          | 16×          |



## Capsula 6 

grep -rn "sync\.\|map\[\|os.Create\|ioutil.WriteFile\|time.Ticker" internal/ cmd/

internal/scouter/store.go:20:   mu      sync.RWMutex
internal/scouter/store.go:21:   byFight map[string][]Reading
internal/scouter/store.go:25:   return &MemStore{byFight: make(map[string][]Reading)}
internal/scouter/server_test.go:45:     for name, body := range map[string]string{
internal/scouter/server.go:75:  writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
internal/scouter/server.go:80:          writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "draining"})
internal/scouter/server.go:83:  writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
internal/scouter/server.go:152: writeJSON(w, code, map[string]string{"error": msg})
internal/scouter/server.go:183: writeJSON(w, http.StatusOK, map[string]any{


## Capsula 8 
La auth en el borde es tan fuerte como el aislamiento de red que hay detrás.