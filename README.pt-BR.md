# DataGuardian

[Read in English](README.md) · O inglês é o idioma principal e canônico do projeto.

DataGuardian é uma plataforma local, Docker-first, para inspeção passiva de arquivos e URLs suspeitos. Ela reduz a exposição antes do download ou abertura local, apresenta achados determinísticos e explica a classificação de risco.

> O DataGuardian não é antivírus, sandbox, detonação de malware, navegador remoto, scanner de vulnerabilidades ou ferramenta de exploração. Ele não garante que um arquivo seja seguro.

## Funcionalidades

- upload autenticado de PDF, JPEG, PNG e texto;
- validação de MIME, extensão, tamanho e nome;
- análise passiva de bytes, sem execução;
- URLs HTTP/HTTPS com proteção SSRF em todos os redirects;
- bloqueio de localhost, loopback, redes privadas e IPs não globais;
- limites de timeout, tamanho e redirects;
- detecção determinística de marcadores PDF, `eval(` e strings codificadas;
- extração limitada de metadados PDF e EXIF;
- risco LOW, MEDIUM ou HIGH com evidências;
- pré-visualizações estáticas e não executáveis;
- inspeção de arquivos remotos antes do download local;
- cópias com remoção de metadados suportados, preservando o original;
- histórico, filtros, paginação, exclusão e exports JSON/PDF;
- tema e idioma persistidos somente no navegador;
- atalhos para Windows, Linux e macOS.

## Segurança

Todo conteúdo é tratado como não confiável. O sistema nunca deve executar arquivos, JavaScript de PDF ou páginas remotas; confiar apenas em `Content-Type`; seguir redirect sem validação; expor caminhos internos; sobrescrever originais; ou afirmar que sanitização remove malware.

Cópias sanitizadas removem apenas metadados suportados e ainda podem conter scripts, objetos incorporados ou malware.

## Início rápido

Requisitos: Docker Desktop no Windows/macOS ou Docker Engine no Linux, Docker Compose, `curl` e portas 3000, 8000 e 5434 livres.

Linux/macOS:

```bash
./start-dataguardian.sh
```

Windows:

```bat
start-dataguardian.bat
```

Ou:

```bash
docker compose up -d --build db backend-go frontend
```

Acesse [http://localhost:3000](http://localhost:3000). Usuários apenas para desenvolvimento/teste: `admin/admin123` e `test/test123`.

## Atalhos

```powershell
# Windows
powershell -ExecutionPolicy Bypass -File .\scripts\desktop-shortcut.ps1 install
```

```bash
# Linux
./scripts/desktop-shortcut.sh install

# macOS
./scripts/desktop-shortcut-macos.sh install
```

Use `remove` para desinstalar. Os helpers atuam somente no usuário atual, não exigem administrador e não instalam Docker silenciosamente.

| Sistema | Runtime | Validação |
|---|---|---|
| Windows 10/11 | Docker Desktop | Sintaxe validada; teste manual recomendado |
| macOS Intel/Apple Silicon | Docker Desktop | Sintaxe validada; teste manual recomendado |
| Linux amd64/arm64 | Docker Engine/Compose | Fluxo e atalhos testados |

## Idioma

Inglês é o padrão. Português do Brasil pode ser selecionado nas telas principais. A preferência fica no `localStorage`, não é enviada ao backend e pode ser alterada a qualquer momento.

## Uso

1. Entre com uma conta.
2. Crie ou selecione um projeto.
3. Envie arquivo ou URL.
4. Revise risco, achados e metadados.
5. Use a pré-visualização estática.
6. Decida se deseja baixar o original.
7. Trate a cópia sanitizada como remoção de metadados, não como arquivo seguro.

## Amostras seguras

[`samples/`](samples/) contém arquivos limpos, marcadores suspeitos inertes, EXIF/GPS fictício e casos rejeitados. Nenhuma amostra contém exploit, macro, backdoor ou código executável.

```bash
python3 scripts/generate_safe_samples.py --check
cd samples && sha256sum -c CHECKSUMS.sha256
```

## Arquitetura

```text
Browser → Next.js :3000 → Go API :8000 → PostgreSQL
                              ├─ análise determinística
                              ├─ fetch passivo com SSRF
                              └─ volume isolado
```

O fluxo é síncrono e compacto. Não existem filas ocultas, segundo backend ou pipeline paralelo.

## API

- autenticação: `/auth/login`, `/auth/register`, `/auth/logout`, `/auth/me`;
- projetos: `/projects`, `/projects/{id}`, `/projects/{id}/audits`;
- análises: `/analyses`, `/analyses/{id}`;
- downloads: `/analyses/{id}/file`, `/analyses/{id}/clean-file`;
- administração: `/storage`;
- saúde: `/health`.

Rotas sensíveis exigem autenticação e validação de propriedade.

## Configuração

Use `.env.example` como referência. Nunca faça commit de `.env`, chaves, tokens, certificados, bancos, backups ou arquivos reais analisados. Produção exige banco, chaves JWT e configurações de segurança explícitas.

## Testes

```bash
cd backend-go && go test ./... && go vet ./...
cd ../frontend && npm ci && npm run test && npm run build
cd .. && ./security/run_security_checks.sh
```

Com a stack iniciada:

```bash
./scripts/smoke.sh
```

O CI executa testes, build, auditoria npm, scan de segredos, CodeQL e Docker multi-arquitetura. Tags `v*` podem criar release draft com checksum e SBOM.

## Backup, releases e empacotamento

- [`docs/BACKUP-RESTORE.md`](docs/BACKUP-RESTORE.md)
- [`docs/RELEASES.md`](docs/RELEASES.md)
- [`docs/PACKAGING.md`](docs/PACKAGING.md)

Backups podem conter material sensível ou malicioso. Docker Compose permanece o único runtime; os atalhos apenas chamam os launchers existentes.

## Limitações

- regras por marcadores podem gerar falsos positivos/negativos;
- conteúdo comprimido, ofuscado ou criptografado pode não ser detectado;
- PDF/EXIF cobrem um subconjunto;
- preview PDF não é renderização completa;
- sanitização não é desarmamento;
- análise de URL não substitui reputação, sandbox ou revisão humana;
- rate limit é local ao processo;
- suporte multiplataforma depende do Docker.

## Estrutura

```text
backend-go/  backend e analisadores
frontend/    aplicação Next.js
samples/     fixtures seguras
scripts/     launchers, smoke e atalhos
security/    verificações
tests/       contratos e E2E
docs/        operação e screenshots
.github/     CI e templates
```

## Contribuição e segurança

Leia [`CONTRIBUTING.md`](CONTRIBUTING.md), [`AGENTS.md`](AGENTS.md) e [`SECURITY.md`](SECURITY.md). Relate vulnerabilidades de forma privada via GitHub Security Advisories.

## Portfólio

O projeto demonstra Go, Next.js, PostgreSQL, autenticação/autorização, SSRF, conteúdo não confiável, análise explicável, Docker, CI/CD, CodeQL, SBOM, acessibilidade, internacionalização e documentação de limitações.

## Licença

Consulte [`LICENSE`](LICENSE).
