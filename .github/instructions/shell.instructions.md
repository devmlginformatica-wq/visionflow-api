---
description: "Use when changing the authenticated UI shell's company, regional, contact or logged-in user data in internal/features/shell."
---
# Shell da interface

- Preserve `GET /shell/empresa` e `GET /shell/notificacoes` como superfícies de suporte à sessão da interface.
- Restrinja empresa e usuário ao tenant, empresa e usuário autenticados; retorne apenas dados ativos e não use este endpoint para buscar dados de outra empresa.
- Preserve o formato de empresa e o objeto aninhado `usuario_logado` consumidos pelo front-end.
- Mantenha o service responsável pela sessão RLS e o repository pela consulta; traduza ausência de registros pelo tratamento HTTP existente.
