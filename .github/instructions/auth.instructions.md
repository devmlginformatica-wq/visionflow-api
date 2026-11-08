---
description: "Use when changing login, authentication, JWT creation or protected API routes in internal/features/auth or internal/httpapi."
---
# Autenticação

- Mantenha a autenticação no módulo `internal/features/auth`; preserve a divisão entre handler, service e repository.
- Preserve o contrato `POST /auth/login` e a resposta de erro genérica para credenciais inválidas. Não revele se o e-mail ou a senha falhou.
- Valide usuário, tenant e empresa ativos e a associação em `usuario_empresa`; mantenha a verificação de senha via PostgreSQL `crypt`.
- Preserve o token JWT e o prazo de oito horas definido pelo contrato HTTP. Não registre senhas, hashes, tokens ou outras credenciais.
- Cubra login válido, credenciais inválidas e falhas de infraestrutura com testes. Para o fluxo integrado, use `docs/api-tests.html` no navegador.
