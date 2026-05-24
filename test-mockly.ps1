# Mockly Real-User Test Script
# Run this in PowerShell: .\test-mockly.ps1

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Mockly - Real User Test Suite" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Check if mockly.exe exists
if (-not (Test-Path ".\mockly.exe")) {
    Write-Host "[ERROR] mockly.exe not found. Build it first with: go build -o mockly.exe ./cmd/mockly" -ForegroundColor Red
    exit 1
}

# Check if mockly.json exists
if (-not (Test-Path ".\mockly.json")) {
    Write-Host "[ERROR] mockly.json not found in current directory" -ForegroundColor Red
    exit 1
}

# Kill any existing mockly process on port 3000
Write-Host "[INFO] Checking for existing mockly processes..." -ForegroundColor Yellow
$existing = Get-Process -Name "mockly" -ErrorAction SilentlyContinue
if ($existing) {
    Write-Host "[INFO] Stopping existing mockly process..." -ForegroundColor Yellow
    $existing | Stop-Process -Force
    Start-Sleep -Seconds 1
}

# Remove old database
if (Test-Path ".\mockly.db") {
    Remove-Item ".\mockly.db" -Force -ErrorAction SilentlyContinue
    Remove-Item ".\mockly.db-wal" -Force -ErrorAction SilentlyContinue
    Remove-Item ".\mockly.db-shm" -Force -ErrorAction SilentlyContinue
    Write-Host "[INFO] Cleaned up old database" -ForegroundColor Yellow
}

# Start the server
Write-Host "[INFO] Starting Mockly server..." -ForegroundColor Green
$process = Start-Process -FilePath ".\mockly.exe" -ArgumentList @("up", "--port", "3000", "--seed", "50") -NoNewWindow -PassThru -RedirectStandardOutput "mockly-server.log" -RedirectStandardError "mockly-server-err.log"

Write-Host "[INFO] Server PID: $($process.Id)" -ForegroundColor Green

# Wait for server to start
Write-Host "[INFO] Waiting for server to start..." -ForegroundColor Yellow
Start-Sleep -Seconds 3

# Helper function for API calls
function Test-Endpoint {
    param($Method, $Url, $Body, $Description)
    
    Write-Host "" 
    Write-Host "--- $Description ---" -ForegroundColor Magenta
    
    try {
        if ($Method -eq "GET") {
            $response = Invoke-WebRequest -Uri $Url -UseBasicParsing -Method Get
        } elseif ($Method -eq "POST") {
            $response = Invoke-WebRequest -Uri $Url -UseBasicParsing -Method Post -Body $Body -ContentType "application/json"
        } elseif ($Method -eq "DELETE") {
            $response = Invoke-WebRequest -Uri $Url -UseBasicParsing -Method Delete
        }
        
        $content = $response.Content
        
        if ($response.StatusCode -ge 200 -and $response.StatusCode -lt 300) {
            Write-Host "[PASS] Status: $($response.StatusCode)" -ForegroundColor Green
        } else {
            Write-Host "[FAIL] Status: $($response.StatusCode)" -ForegroundColor Red
        }
        
        # Try to pretty-print JSON
        try {
            $json = $content | ConvertFrom-Json
            Write-Host "Response:" -ForegroundColor Gray
            Write-Host ($json | ConvertTo-Json -Depth 10) -ForegroundColor White
        } catch {
            Write-Host "Response: $content" -ForegroundColor White
        }
        
        return $content
    } catch {
        Write-Host "[FAIL] Error: $($_.Exception.Message)" -ForegroundColor Red
        return $null
    }
}

# ========== TESTS ==========

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  RUNNING TESTS" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

# Test 1: Health Check
Test-Endpoint -Method GET -Url "http://localhost:3000/api/health" -Description "Test 1: Health Check"

# Test 2: Root Endpoint
Test-Endpoint -Method GET -Url "http://localhost:3000/" -Description "Test 2: Root Endpoint (list endpoints)"

# Test 3: List all users
Test-Endpoint -Method GET -Url "http://localhost:3000/api/users" -Description "Test 3: List all users"

# Test 4: List all products
Test-Endpoint -Method GET -Url "http://localhost:3000/api/products" -Description "Test 4: List all products"

# Test 5: Pagination - first 5 users
Test-Endpoint -Method GET -Url "http://localhost:3000/api/users?limit=5&offset=0" -Description "Test 5: Pagination (first 5 users)"

# Test 6: Pagination - next 5 users
Test-Endpoint -Method GET -Url "http://localhost:3000/api/users?limit=5&offset=5" -Description "Test 6: Pagination (next 5 users)"

# Test 7: Get single user by ID (get the first user's ID first)
try {
    $usersResponse = Invoke-WebRequest -Uri "http://localhost:3000/api/users?limit=1" -UseBasicParsing -Method Get
    $firstUser = ($usersResponse.Content | ConvertFrom-Json)[0]
    if ($firstUser) {
        $userId = $firstUser.id
        Test-Endpoint -Method GET -Url "http://localhost:3000/api/users/$userId" -Description "Test 7: Get single user by ID ($userId)"
    }
} catch {
    Write-Host "[FAIL] Test 7: Could not fetch user ID" -ForegroundColor Red
}

# Test 8: Create a new product (POST)
$newProduct = @{
    name = "Test Product"
    price = 29.99
    in_stock = $true
} | ConvertTo-Json
Test-Endpoint -Method POST -Url "http://localhost:3000/api/products" -Body $newProduct -Description "Test 8: Create new product (POST)"

# Test 9: Get the created product by ID
try {
    $productsResponse = Invoke-WebRequest -Uri "http://localhost:3000/api/products?limit=100" -UseBasicParsing -Method Get
    $allProducts = $productsResponse.Content | ConvertFrom-Json
    $lastProduct = $allProducts[-1]
    if ($lastProduct) {
        $productId = $lastProduct.id
        Test-Endpoint -Method GET -Url "http://localhost:3000/api/products/$productId" -Description "Test 9: Get created product by ID ($productId)"
    }
} catch {
    Write-Host "[FAIL] Test 9: Could not verify created product" -ForegroundColor Red
}

# Test 10: Delete a product (delete the last one)
try {
    $productsResponse2 = Invoke-WebRequest -Uri "http://localhost:3000/api/products?limit=100" -UseBasicParsing -Method Get
    $allProducts2 = $productsResponse2.Content | ConvertFrom-Json
    $delProduct = $allProducts2[-1]
    if ($delProduct) {
        $deleteId = $delProduct.id
        Test-Endpoint -Method DELETE -Url "http://localhost:3000/api/products/$deleteId" -Description "Test 10: Delete product by ID ($deleteId)"
    }
} catch {
    Write-Host "[FAIL] Test 10: Could not delete product" -ForegroundColor Red
}

# Test 11: Verify deletion - product should return 404
try {
    $productsResponse3 = Invoke-WebRequest -Uri "http://localhost:3000/api/products?limit=100" -UseBasicParsing -Method Get
    $allProducts3 = $productsResponse3.Content | ConvertFrom-Json
    $deletedCheck = $allProducts3 | Where-Object { $_.id -eq $deleteId }
    if ($deletedCheck) {
        Write-Host "[FAIL] Test 11: Product still exists after deletion!" -ForegroundColor Red
    } else {
        Write-Host "[PASS] Test 11: Product successfully deleted (not in list)" -ForegroundColor Green
    }
} catch {
    Write-Host "[FAIL] Test 11: Error checking deletion" -ForegroundColor Red
}

# Test 12: POST a new user and verify it gets auto-assigned UUID + timestamp
$newUser = @{
    name = "Jane Doe"
    email = "jane@example.com"
} | ConvertTo-Json
Test-Endpoint -Method POST -Url "http://localhost:3000/api/users" -Body $newUser -Description "Test 12: Create user (auto UUID + timestamp)"

# Test 13: Health check after all operations
Test-Endpoint -Method GET -Url "http://localhost:3000/api/health" -Description "Test 13: Health check (final)"

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  TEST RESULTS" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "[DONE] All tests completed. Check results above." -ForegroundColor Cyan
Write-Host ""
Write-Host "Server log file: mockly-server.log" -ForegroundColor Gray
Write-Host ""
Write-Host "To stop the server, run:" -ForegroundColor Yellow
Write-Host "  Stop-Process -Id $($process.Id) -Force" -ForegroundColor White
Write-Host ""

# Keep the process reference for cleanup
$process