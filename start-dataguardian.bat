@echo off
setlocal EnableExtensions EnableDelayedExpansion

set "ROOT_DIR=%~dp0"
set "APP_URL=http://localhost:3000"

cd /d "%ROOT_DIR%"

echo [DataGuardian] Starting DataGuardian
echo [DataGuardian] This helper only runs Docker Compose commands in this folder.
echo [DataGuardian] It does not install services or change system settings.
echo [DataGuardian] This may take a few minutes the first time while Docker builds images.

where docker >nul 2>nul
if errorlevel 1 (
  echo [DataGuardian] Docker was not found.
  echo [DataGuardian] Install Docker Desktop, then run this launcher again:
  echo [DataGuardian] https://docs.docker.com/desktop/setup/install/windows-install/
  pause
  exit /b 1
)

docker compose version >nul 2>nul
if errorlevel 1 (
  echo [DataGuardian] Docker Compose is not available.
  echo [DataGuardian] Install or update Docker Desktop, then run this launcher again.
  pause
  exit /b 1
)

docker info >nul 2>nul
if errorlevel 1 (
  goto docker_unavailable
)

call :check_port 8000 "backend health" "http://localhost:8000/health"
if errorlevel 1 goto port_problem

call :check_port 3000 "frontend login" "http://localhost:3000/login"
if errorlevel 1 goto port_problem

call :check_tcp_port 5434
if errorlevel 1 goto port_problem

echo [DataGuardian] Starting Docker Compose services: db backend-go frontend
echo [DataGuardian] Running: docker compose up -d --build db backend-go
docker compose up -d --build db backend-go
if errorlevel 1 goto startup_failed

echo [DataGuardian] Running: docker compose up -d --build --force-recreate frontend
docker compose up -d --build --force-recreate frontend
if errorlevel 1 goto startup_failed

call :wait_for_url "backend" "http://localhost:8000/health" 60
if errorlevel 1 goto health_failed

call :wait_for_url "frontend" "http://localhost:3000/login" 75
if errorlevel 1 goto health_failed

choice /C YN /N /M "[DataGuardian] Open DataGuardian in your browser now? [Y/N]: "
if errorlevel 2 goto browser_skipped
if errorlevel 1 (
  echo [DataGuardian] Opening %APP_URL%
  start "" "%APP_URL%"
)

:browser_skipped
echo.
echo [DataGuardian] DataGuardian started correctly.
echo [DataGuardian] App: %APP_URL%
echo [DataGuardian] Backend health: http://localhost:8000/health
echo [DataGuardian] Demo users: admin / admin123 or test / test123
echo.
pause
exit /b 0

:check_port
set "PORT=%~1"
set "LABEL=%~2"
set "URL=%~3"
netstat -ano -p tcp | findstr /R /C:":%PORT% .*LISTENING" >nul 2>nul
if errorlevel 1 exit /b 0

curl -fsS --max-time 3 "%URL%" >nul 2>nul
if not errorlevel 1 (
  echo [DataGuardian] %LABEL% already appears to be running on port %PORT%; reusing it.
  exit /b 0
)

echo [DataGuardian] Port %PORT% is already in use by another program.
echo [DataGuardian] Close the program using port %PORT%, then run this launcher again.
exit /b 1

:check_tcp_port
set "PORT=%~1"
netstat -ano -p tcp | findstr /R /C:":%PORT% .*LISTENING" >nul 2>nul
if errorlevel 1 exit /b 0

docker ps --filter "name=dataguardian_db" --format "{{.Names}}" | findstr /X "dataguardian_db" >nul 2>nul
if not errorlevel 1 (
  echo [DataGuardian] DataGuardian database already appears to be running on port %PORT%; reusing it.
  exit /b 0
)

echo [DataGuardian] Port %PORT% is already in use by another program.
echo [DataGuardian] Close the program using port %PORT%, then run this launcher again.
exit /b 1

:wait_for_url
set "NAME=%~1"
set "URL=%~2"
set "SECONDS=%~3"
for /L %%I in (1,1,%SECONDS%) do (
  curl -fsS --max-time 3 "%URL%" >nul 2>nul
  if not errorlevel 1 (
    echo [DataGuardian] %NAME% is ready.
    exit /b 0
  )
  timeout /t 1 /nobreak >nul
)
echo [DataGuardian] %NAME% did not become ready at %URL%.
exit /b 1

:docker_unavailable
echo [DataGuardian] Docker is installed, but the Docker daemon is not reachable.
echo [DataGuardian] Start Docker Desktop yourself, wait until it says Docker is running, then run this launcher again.
echo [DataGuardian] No Docker process was started by this helper.
pause
exit /b 1

:port_problem
echo.
echo [DataGuardian] Startup stopped because a required port is busy.
echo [DataGuardian] DataGuardian uses ports 3000, 8000, and 5434.
echo [DataGuardian] Close the other program, or run this command to inspect containers:
echo [DataGuardian] docker compose ps
pause
exit /b 1

:startup_failed
echo.
echo [DataGuardian] Docker Compose could not start the stack.
echo [DataGuardian] Try: docker compose down
echo [DataGuardian] Then run this launcher again.
echo [DataGuardian] For details: docker compose logs backend-go frontend
pause
exit /b 1

:health_failed
echo.
echo [DataGuardian] The containers started, but the app did not become ready in time.
echo [DataGuardian] Try: docker compose logs backend-go frontend
echo [DataGuardian] Then run: docker compose down
echo [DataGuardian] After that, run this launcher again.
pause
exit /b 1
