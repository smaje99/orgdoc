
Actúa como arquitecto de software senior, tech lead Python y especialista en construcción de herramientas CLI instalables para Linux/Ubuntu.

Quiero construir una aplicación CLI en Python que pueda instalar en mi sistema operativo Ubuntu y ejecutar mediante un comando global cuando lo requiera.

Nombre tentativo del proyecto:
media-report-cli

Propósito general:
Crear una herramienta CLI que, dada una carpeta o archivo de entrada con video o audio, automatice el flujo completo:

1. Detectar archivos de video/audio compatibles.
2. Extraer audio si el archivo es video.
3. Normalizar audio para transcripción.
4. Transcribir con Whisper local o backend configurable.
5. Guardar transcripción cruda y estructurada.
6. Limpiar y segmentar el texto.
7. Enviar la transcripción a un LLM configurable para generar un informe.
8. Generar informe en Markdown.
9. Generar PDF usando Pandoc + LaTeX.
10. Dejar todos los archivos intermedios y finales en una carpeta de artefactos creada junto al archivo original.

Restricción importante:
La herramienta debe correr principalmente en Ubuntu/Linux. No debe depender de una interfaz gráfica. Debe funcionar como CLI instalable.

Ejemplo de uso esperado:

media-report process ./reunion.mp4

media-report process ./carpeta-con-videos --recursive

media-report process ./clase.mp3 --provider openai --model gpt-4.1-mini --language es

media-report process ./entrevista.mp4 --provider ollama --model llama3.1 --template meeting

media-report process ./video.mp4 --only-transcribe

media-report process ./audio.wav --only-report

media-report doctor

media-report templates list

media-report config show

media-report config init

Requisitos funcionales principales:

1. CLI
- Usar Typer como framework CLI.
- Usar Rich para logs, barras de progreso y mensajes claros.
- El comando principal debe llamarse `media-report`.
- Debe poder procesar un archivo individual o una carpeta.
- Debe aceptar procesamiento recursivo.
- Debe permitir configurar idioma, proveedor LLM, modelo, plantilla de informe, salida PDF/Markdown y conservación de temporales.
- Por defecto, debe conservar temporales.
- Debe tener comando `doctor` para verificar dependencias externas: ffmpeg, pandoc, xelatex/lualatex, Python, acceso a backend de Whisper y configuración de API keys.

2. Entrada
- Soportar archivos de video: mp4, mkv, mov, webm, avi.
- Soportar archivos de audio: mp3, wav, m4a, flac, ogg.
- Validar existencia de rutas.
- No sobrescribir resultados anteriores sin confirmación o sin flag `--overwrite`.
- Crear una carpeta de artefactos junto al archivo original con este patrón:
  <nombre_archivo>_media_report/
- Dentro de esa carpeta guardar:
  - metadata.json
  - pipeline.log
  - audio_extracted.wav
  - audio_normalized.wav
  - transcript_raw.txt
  - transcript_segments.json
  - transcript_clean.md
  - report.md
  - report.pdf
  - prompt_used.md
  - llm_response_raw.txt

3. Extracción y normalización de audio
- Usar FFmpeg mediante subprocess controlado.
- Extraer audio con:
  - mono
  - 16 kHz
  - WAV
- Centralizar comandos FFmpeg en un servicio propio.
- Capturar errores de FFmpeg con mensajes entendibles.

4. Transcripción
- Implementar una interfaz/puerto `TranscriptionProvider`.
- Implementar inicialmente `FasterWhisperProvider`.
- Diseñar el sistema para soportar en el futuro:
  - OpenAI Whisper API
  - whisper.cpp
  - WhisperX
- Guardar:
  - texto completo
  - segmentos con timestamps
  - idioma detectado si está disponible
  - duración
  - modelo usado
- Permitir seleccionar modelo local:
  - tiny
  - base
  - small
  - medium
  - large-v3
  - turbo, si está disponible según backend
- Permitir device:
  - auto
  - cpu
  - cuda

5. Generación de informe
- Implementar una interfaz/puerto `LLMProvider`.
- Implementar proveedores iniciales:
  - OpenAI-compatible API
  - Ollama local
- Diseñar para agregar fácilmente:
  - Anthropic
  - Gemini
  - Groq
  - OpenRouter
  - LM Studio
- La configuración debe permitir base_url, api_key, model, temperature, max_tokens y timeout.
- El informe debe generarse a partir de plantillas de prompt.
- Deben existir plantillas iniciales:
  - generic
  - meeting
  - class_notes
  - interview
  - technical_report
- El informe Markdown debe tener estructura profesional:
  - título
  - metadatos
  - resumen ejecutivo
  - temas principales
  - desarrollo organizado
  - decisiones o conclusiones
  - tareas/pendientes si aplica
  - riesgos o dudas si aplica
  - resumen final
- Debe guardarse el prompt usado para trazabilidad.

6. PDF con LaTeX
- Usar Pandoc para convertir Markdown a PDF.
- Usar `xelatex` como motor predeterminado.
- Si falla la generación de PDF, el proceso no debe perder el Markdown.
- Debe mostrar error claro indicando qué dependencia falta.
- Preparar una plantilla básica de Pandoc/LaTeX opcional en:
  templates/pdf/default.tex

7. Configuración
- Usar pydantic-settings.
- Soportar archivo de configuración en:
  ~/.config/media-report/config.toml
- Soportar variables de entorno:
  MEDIA_REPORT_LLM_PROVIDER
  MEDIA_REPORT_LLM_MODEL
  MEDIA_REPORT_OPENAI_API_KEY
  MEDIA_REPORT_OPENAI_BASE_URL
  MEDIA_REPORT_OLLAMA_BASE_URL
  MEDIA_REPORT_WHISPER_MODEL
  MEDIA_REPORT_OUTPUT_FORMAT
- Comando:
  media-report config init
  debe crear un archivo de configuración base.
- Comando:
  media-report config show
  debe mostrar la configuración efectiva ocultando secretos.

8. Arquitectura
Usar arquitectura hexagonal con vertical slicing moderado.

Estructura sugerida:

media_report_cli/
  src/
    media_report/
      __init__.py
      cli/
        app.py
        commands/
          process.py
          doctor.py
          config.py
          templates.py
      core/
        models.py
        errors.py
        settings.py
        logging.py
      domain/
        media/
          entities.py
          services.py
        transcription/
          entities.py
          ports.py
          services.py
        reporting/
          entities.py
          ports.py
          services.py
        artifacts/
          services.py
      infrastructure/
        ffmpeg/
          service.py
        transcription/
          faster_whisper_provider.py
        llm/
          openai_compatible_provider.py
          ollama_provider.py
        document/
          pandoc_service.py
        filesystem/
          scanner.py
      application/
        process_media/
          command.py
          handler.py
          workflow.py
      templates/
        prompts/
          generic.md
          meeting.md
          class_notes.md
          interview.md
          technical_report.md
        pdf/
          default.tex
  tests/
    unit/
    integration/
  docs/
    context/
    architecture/
    usage/
  skills/
  AGENTS.md
  README.md
  pyproject.toml
  .env.example
  .gitignore
  LICENSE

9. Gestión de dependencias y empaquetado
- Usar `uv`.
- Usar `pyproject.toml`.
- Configurar entry point:
  media-report = "media_report.cli.app:app"
- Usar Ruff para linting y formatting.
- Usar Pytest para pruebas.
- Usar `ty` para type checking.
- Incluir Makefile o justfile con comandos:
  install
  dev
  lint
  format
  test
  build
  doctor
- Incluir README con instalación local:
  uv sync
  uv run media-report doctor
  uv tool install .
  media-report process ./archivo.mp4

10. Testing
Crear pruebas unitarias para:
- detección de tipo de archivo
- creación de carpeta de artefactos
- generación de nombres de salida
- construcción de comandos FFmpeg
- lectura de configuración
- selección de proveedor LLM
- renderizado de prompts
- manejo de errores
- flujo de proceso con mocks

Crear pruebas de integración mínimas con archivos dummy o fixtures pequeñas, sin requerir modelos pesados por defecto.

11. Seguridad y privacidad
- No registrar API keys.
- Ocultar secretos en logs.
- No subir archivos a servicios externos salvo que el usuario seleccione explícitamente un proveedor remoto.
- Mostrar advertencia cuando se use proveedor remoto.
- Los archivos se procesan localmente por defecto si se usa faster-whisper + Ollama.
- Incluir sección de privacidad en README.

12. Robustez
- Manejar errores recuperables.
- Si falla PDF, conservar Markdown.
- Si falla LLM, conservar transcripción.
- Si falla transcripción, conservar audio extraído.
- Registrar estado de cada etapa en metadata.json.
- Permitir reanudar en el futuro, aunque para la primera versión basta con diseñar metadata pensando en ello.

13. Context engineering
Antes de generar código, crea una carpeta `docs/context/` con estos documentos:

- product_context.md
  Explica el problema, usuarios objetivo, casos de uso y alcance del producto.

- technical_context.md
  Explica stack, dependencias externas, flujo técnico y restricciones de Ubuntu/Linux.

- architecture_context.md
  Explica arquitectura hexagonal, vertical slicing, puertos/adaptadores y decisiones de diseño.

- workflow_context.md
  Explica paso a paso el pipeline: entrada, audio, transcripción, limpieza, informe, PDF y artefactos.

- llm_context.md
  Explica cómo deben funcionar los proveedores LLM, contratos de prompt, trazabilidad y manejo de errores.

- testing_context.md
  Explica estrategia de pruebas unitarias, integración, mocks y fixtures.

- roadmap.md
  Divide el desarrollo en fases:
  Fase 0: Bootstrap del proyecto.
  Fase 1: CLI base + configuración + doctor.
  Fase 2: detección de medios + artefactos.
  Fase 3: FFmpeg.
  Fase 4: transcripción.
  Fase 5: generación Markdown con LLM.
  Fase 6: PDF con Pandoc/LaTeX.
  Fase 7: pruebas, documentación y empaquetado.
  Fase 8: mejoras futuras como diarización, WhisperX, cola de trabajos, interfaz TUI, watcher de carpetas.

14. AGENTS.md
Genera un archivo `AGENTS.md` en la raíz del proyecto con:
- propósito del proyecto
- arquitectura obligatoria
- comandos de desarrollo
- reglas de estilo
- reglas de testing
- convenciones de commits
- reglas de seguridad
- cómo agregar proveedores LLM
- cómo agregar proveedores de transcripción
- cómo agregar plantillas de informe
- instrucciones para no romper compatibilidad CLI
- definición de terminado para cada tarea

15. Skills
Crea una carpeta `skills/` con skills documentadas para agentes de desarrollo. Cada skill debe tener un archivo Markdown claro, con objetivo, cuándo usarla, pasos, checklist y criterios de salida.

Skills requeridas:

skills/project-planning/SKILL.md
- Para dividir features en tareas pequeñas.
- Debe generar backlog, criterios de aceptación y riesgos.

skills/cli-design/SKILL.md
- Para diseñar comandos, flags, ayuda, errores y UX CLI con Typer/Rich.

skills/hexagonal-architecture/SKILL.md
- Para mantener separación entre dominio, aplicación e infraestructura.

skills/media-processing/SKILL.md
- Para trabajar con FFmpeg, extracción y normalización de audio.

skills/transcription-provider/SKILL.md
- Para implementar o modificar proveedores de transcripción.

skills/llm-provider/SKILL.md
- Para implementar proveedores LLM OpenAI-compatible, Ollama, Gemini, Anthropic, Groq, etc.

skills/report-template-design/SKILL.md
- Para crear plantillas de prompts de informes en Markdown.

skills/pdf-generation/SKILL.md
- Para convertir Markdown a PDF con Pandoc y LaTeX.

skills/testing-strategy/SKILL.md
- Para escribir pruebas unitarias e integración.

skills/release-packaging/SKILL.md
- Para empaquetar, instalar y distribuir la CLI.

16. Actividades iniciales que debes generar
Primero no escribas todo el código de una sola vez. Haz esto en orden:

A. Analiza el alcance.
B. Propón arquitectura final.
C. Genera backlog técnico por fases.
D. Genera árbol de archivos definitivo.
E. Genera `docs/context/*`.
F. Genera `AGENTS.md`.
G. Genera `skills/*/SKILL.md`.
H. Genera pyproject.toml inicial.
I. Genera CLI mínima con comandos:
   - process
   - doctor
   - config init
   - config show
   - templates list
J. Genera pruebas mínimas.
K. Luego implementa progresivamente cada fase.

17. Criterios de aceptación de la primera versión funcional
La primera versión se considera funcional cuando:

- Puedo instalarla localmente con `uv tool install .`
- Puedo ejecutar `media-report doctor`
- Puedo ejecutar `media-report config init`
- Puedo ejecutar `media-report process ./archivo.mp4`
- Se crea una carpeta `<archivo>_media_report/`
- Se extrae el audio si el archivo es video
- Se transcribe el audio con faster-whisper
- Se genera `transcript_raw.txt`
- Se genera `transcript_segments.json`
- Se genera `report.md` usando proveedor LLM configurado
- Se genera `report.pdf` si Pandoc y LaTeX están instalados
- Si alguna etapa falla, quedan guardados los artefactos de las etapas anteriores
- No se imprimen secretos en consola ni logs
- Hay README suficiente para instalar y usar el proyecto

18. Decisiones técnicas iniciales
Usa estas decisiones por defecto salvo que encuentres una razón fuerte para cambiarlas:

- Lenguaje: Python 3.11+
- CLI: Typer
- Consola: Rich
- Configuración: pydantic-settings + TOML
- Empaquetado: uv + pyproject.toml
- Lint/format: Ruff
- Tests: pytest
- Audio/video: FFmpeg
- Transcripción local: faster-whisper
- LLM local: Ollama
- LLM remoto: OpenAI-compatible provider
- PDF: Pandoc + xelatex
- Arquitectura: hexagonal + vertical slicing moderado

19. Reglas de implementación
- No acoples Typer directamente al dominio.
- No mezcles subprocess de FFmpeg en lógica de aplicación.
- No mezcles llamadas LLM con renderizado de prompts.
- No uses API keys hardcodeadas.
- No ocultes errores técnicos en modo verbose.
- Usa errores propios del dominio/aplicación.
- Mantén funciones pequeñas.
- Usa type hints.
- Documenta decisiones relevantes.
- Crea código mantenible, no solo funcional.
- Antes de implementar una fase, actualiza el contexto si hace falta.
- Documenta el código no estrictamente.

20. Resultado esperado de tu primera respuesta
En tu primera respuesta quiero que generes:

1. Diagnóstico breve del alcance.
2. Arquitectura propuesta.
3. Backlog por fases.
4. Árbol de archivos propuesto.
5. Lista de documentos de contexto que crearás.
6. Lista de skills que crearás.
7. Plan de implementación por commits pequeños.
8. Riesgos técnicos y mitigaciones.

Después de esa primera respuesta, comienza a crear los archivos del proyecto siguiendo el plan.

ACTUALIZACIÓN ESTRATÉGICA: DISTRIBUCIÓN COMO PAQUETE PUBLICABLE EN PYPI

El proyecto no debe tratarse únicamente como una herramienta local, sino como una aplicación CLI profesional que vivirá como paquete instalable desde PyPI.

Decisión principal:
- El canal principal de distribución será PyPI.
- El nombre tentativo del paquete será `media-report-cli`.
- El comando global expuesto será `media-report`.
- La instalación recomendada para usuarios finales será mediante:
  - `uv tool install media-report-cli`
  - `pipx install media-report-cli`
- La instalación con `pip install media-report-cli` debe ser posible, pero no debe presentarse como la opción principal para usuarios finales, porque puede mezclar dependencias con el entorno global.

Implicación arquitectónica:
- El proyecto debe diseñarse desde el inicio como paquete Python distribuible.
- No debe depender de rutas relativas frágiles.
- Las plantillas internas de prompts y PDF deben empaquetarse correctamente como package data.
- La CLI debe funcionar después de instalarse globalmente, no solo desde el repositorio en modo desarrollo.
- La lectura de recursos internos debe hacerse con `importlib.resources` o una alternativa moderna y segura, no con rutas hardcodeadas del repositorio.
- El comando `media-report` debe quedar registrado mediante `[project.scripts]` en `pyproject.toml`.

Configuración esperada en `pyproject.toml`:
- Usar `pyproject.toml` como fuente principal de metadatos del paquete.
- Incluir metadatos completos:
  - name
  - version
  - description
  - readme
  - requires-python
  - authors
  - license
  - keywords
  - classifiers
  - dependencies
  - optional-dependencies
  - project.urls
  - project.scripts
- Configurar correctamente el backend de build.
- Preparar el proyecto para construir wheels y sdist.
- Verificar que el paquete pueda instalarse con:
  - `uv tool install .`
  - `pipx install .`
  - `pip install .`

Estructura de distribución:
- El código fuente debe vivir bajo estructura `src/`.
- El paquete importable debe ser `media_report`.
- El repositorio puede llamarse `media-report-cli`.
- El paquete en PyPI debe llamarse `media-report-cli`.
- El comando CLI debe llamarse `media-report`.

Ejemplo esperado de `pyproject.toml`:

[project]
name = "media-report-cli"
version = "0.1.0"
description = "CLI para transformar video/audio en transcripciones, informes Markdown y PDF con IA."
readme = "README.md"
requires-python = ">=3.11"
authors = [
  { name = "Sergio Andrés Majé Franco" }
]
license = { text = "MIT" }
keywords = [
  "cli",
  "whisper",
  "ffmpeg",
  "markdown",
  "latex",
  "pandoc",
  "llm",
  "transcription",
  "automation",
  "documentation"
]
classifiers = [
  "Development Status :: 3 - Alpha",
  "Environment :: Console",
  "Intended Audience :: Developers",
  "Intended Audience :: Information Technology",
  "License :: OSI Approved :: MIT License",
  "Operating System :: POSIX :: Linux",
  "Operating System :: MacOS",
  "Programming Language :: Python :: 3",
  "Programming Language :: Python :: 3.11",
  "Programming Language :: Python :: 3.12",
  "Topic :: Multimedia :: Sound/Audio :: Speech",
  "Topic :: Text Processing",
  "Topic :: Utilities"
]
dependencies = [
  "typer>=0.12",
  "rich>=13.0",
  "pydantic>=2.0",
  "pydantic-settings>=2.0",
  "httpx>=0.27",
  "tomli-w>=1.0"
]

[project.optional-dependencies]
transcription = [
  "faster-whisper>=1.0"
]
dev = [
  "pytest>=8.0",
  "ruff>=0.6",
  "build>=1.2",
  "twine>=5.0",
  "ty">=0.0
]

[project.urls]
Homepage = "https://github.com/smaje99/orgdoc"
Repository = "https://github.com/smaje99/orgdoc"
Issues = "https://github.com/smaje99/orgdoc/issues"

[project.scripts]
media-report = "media_report.cli.app:app"

Dependencias externas:
Aunque el paquete viva en PyPI, NO se deben empaquetar dependencias externas del sistema como:
- ffmpeg
- pandoc
- xelatex / texlive
- ollama
- CUDA
- drivers de GPU

Estas dependencias deben verificarse con:

media-report doctor

El comando `doctor` debe:
- verificar si ffmpeg existe;
- verificar si pandoc existe;
- verificar si xelatex o lualatex existe;
- verificar si Ollama está disponible cuando el proveedor configurado sea Ollama;
- verificar si la API key existe cuando se use proveedor remoto;
- verificar si faster-whisper está instalado cuando se use transcripción local;
- mostrar instrucciones de instalación por sistema operativo cuando falte algo.

Instalación documentada:
El README debe incluir una sección clara de instalación:

1. Instalación recomendada con uv:

uv tool install media-report-cli

2. Instalación recomendada con pipx:

pipx install media-report-cli

3. Instalación desde repositorio local para desarrollo:

git clone https://github.com/smaje99/orgdoc
cd orgdoc
uv sync
uv run media-report doctor

4. Instalación local como herramienta:

uv tool install .

5. Verificación:

media-report doctor

Documentación para PyPI:
El README debe estar escrito pensando en que será mostrado en PyPI.
Debe incluir:
- descripción breve;
- características principales;
- instalación;
- dependencias externas;
- ejemplos de uso;
- configuración;
- privacidad;
- proveedores soportados;
- plantillas de informe;
- roadmap;
- estado del proyecto;
- licencia.

Versionamiento:
- Usar versionamiento semántico.
- Primera versión: 0.1.0.
- No prometer estabilidad de API antes de 1.0.0.
- Documentar cambios en `CHANGELOG.md`.
- Agregar `RELEASE.md` o sección en `docs/release.md` con el flujo de publicación.

Flujo de release esperado:
Crear documentación para publicar en PyPI:

1. Ejecutar pruebas:
   uv run pytest

2. Ejecutar lint:
   uv run ruff check .
   uv run ruff format --check .

3. Construir paquete:
   uv run python -m build

4. Verificar artefactos:
   uv run twine check dist/*

5. Publicar primero en TestPyPI:
   uv run twine upload --repository testpypi dist/*

6. Probar instalación desde TestPyPI:
   uv tool install --index-url https://test.pypi.org/simple/ media-report-cli

7. Publicar en PyPI:
   uv run twine upload dist/*

Criterios de aceptación adicionales por distribución:
La versión base no estará completa hasta que:

- `uv tool install .` instale correctamente el comando `media-report`.
- `pipx install .` instale correctamente el comando `media-report`.
- `media-report --help` funcione fuera del repositorio.
- `media-report templates list` pueda leer las plantillas empaquetadas.
- `media-report process` pueda usar plantillas empaquetadas después de instalación global.
- Los archivos Markdown, LaTeX y prompts incluidos en el paquete estén disponibles mediante package data.
- El README renderice correctamente en PyPI.
- El paquete pueda construirse como wheel y sdist.
- `twine check dist/*` pase sin errores.
- El proyecto tenga `CHANGELOG.md`.
- El proyecto tenga documentación de release.

Cambios en AGENTS.md:
Actualizar `AGENTS.md` para incluir reglas de distribución:
- No introducir rutas que solo funcionen dentro del repositorio.
- Toda plantilla interna debe cargarse como recurso empaquetado.
- Cualquier cambio que afecte CLI debe mantener compatibilidad o documentar breaking change.
- Toda nueva dependencia debe justificarse pensando en instalación desde PyPI.
- No agregar dependencias externas pesadas como obligatorias si pueden ser extras opcionales.
- Los proveedores pesados deben instalarse como extras cuando sea posible.

Extras recomendados:
Diseñar extras opcionales desde el inicio:

media-report-cli[transcription]
media-report-cli[dev]
media-report-cli[all]

Donde:
- `[transcription]` instala faster-whisper.
- `[dev]` instala herramientas de desarrollo.
- `[all]` instala todos los extras Python disponibles, pero nunca instala ffmpeg, pandoc ni LaTeX porque son dependencias del sistema.

Nueva tarea obligatoria:
Además de los documentos contextuales anteriores, crear:

docs/context/distribution_context.md

Este documento debe explicar:
- por qué el proyecto se distribuye por PyPI;
- diferencia entre paquete, comando y repositorio;
- estrategia de instalación global;
- manejo de dependencias externas;
- package data;
- extras opcionales;
- flujo de release;
- riesgos de distribución multiplataforma;
- política inicial de compatibilidad.

Nueva skill obligatoria:
Crear también:

skills/pypi-packaging/SKILL.md

Esta skill debe incluir:
- objetivo;
- cuándo usarla;
- checklist de empaquetado;
- checklist de package data;
- checklist de instalación global;
- checklist de release;
- validaciones con uv, pipx, build y twine;
- errores frecuentes;
- criterios de salida.

Regla final:
A partir de esta actualización, cualquier decisión técnica debe evaluarse también con esta pregunta:

¿Esto funcionará correctamente cuando el usuario instale `media-report-cli` desde PyPI y ejecute `media-report` desde cualquier carpeta del sistema?

---

Revisa el repositorio: https://github.com/firecrawl/pdf-inspector
Corroborar que tal útil puede ser.
