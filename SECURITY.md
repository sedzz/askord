# Política de seguridad

## Cómo informar de una vulnerabilidad

Si encuentras un fallo de seguridad en Askord, **no abras una issue pública**. Usa el aviso privado de GitHub: pestaña **Security** del repositorio → **Report a vulnerability**.

Incluye, si puedes:

- Qué has encontrado y qué impacto tiene.
- Los pasos para reproducirlo.
- La versión o el commit en el que lo viste.

Intentaré responderte en unos días. Pido que no se publique el fallo hasta que haya una corrección.

## Qué se considera en alcance

- Acceso a datos de otros servidores o tickets sin permiso.
- Fallos de autenticación o de sesión en el panel.
- Fugas de las API keys de los LLM, que se guardan cifradas, o de los tokens de Discord.
- Saltarse la protección contra SSRF del cliente del LLM.
- Inyección (SQL, XSS) en la API o en el panel.

## Despliegue

Si alojas Askord tú mismo, mantén `COOKIE_SECURE=true` con HTTPS, `AI_ALLOW_PRIVATE_URLS=false` y guarda `.env` y `AI_ENCRYPTION_KEY` fuera del repositorio.
