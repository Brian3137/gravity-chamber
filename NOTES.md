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