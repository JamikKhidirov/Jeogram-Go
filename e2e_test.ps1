$ErrorActionPreference = "Stop"
$base = "http://localhost:8081"

function Call {
    param($method, $path, $body=$null, $token=$null)
    $h = @{"Content-Type"="application/json"}
    if ($token) { $h["Authorization"] = "Bearer $token" }
    try {
        if ($body) {
            $resp = Invoke-RestMethod -Method $method -Uri "$base$path" -Headers $h -Body ($body | ConvertTo-Json -Depth 5)
        } else {
            $resp = Invoke-RestMethod -Method $method -Uri "$base$path" -Headers $h
        }
        Write-Host ("  [$method $path] -> OK")
        return $resp
    } catch {
        $code = $_.Exception.Response.StatusCode.value__
        $msg = $_.ErrorDetails.Message
        Write-Host ("  [$method $path] -> HTTP $code :: $msg")
        return $null
    }
}

Write-Host "=== HEALTH / SWAGGER / METRICS ==="
(Invoke-WebRequest -Uri "$base/health" -ErrorAction SilentlyContinue).StatusCode
(Invoke-WebRequest -Uri "$base/swagger/index.html" -ErrorAction SilentlyContinue).StatusCode
(Invoke-WebRequest -Uri "$base/swagger/doc.json" -ErrorAction SilentlyContinue).StatusCode
(Invoke-WebRequest -Uri "$base/metrics" -ErrorAction SilentlyContinue).StatusCode

Write-Host "=== NO-AUTH SHOULD 401 ==="
Call -method GET -path "/chats"

Write-Host "=== REGISTER A & B ==="
$a = Call -method POST -path "/auth/register" -body @{email="alice@example.com"; username="alice"; password="password123"}
$b = Call -method POST -path "/auth/register" -body @{email="bob@example.com"; username="bob"; password="password123"}
$tokenA = $a.data.access_token
$tokenB = $b.data.access_token
$idA = $a.data.user.id
$idB = $b.data.user.id
Write-Host "  A id=$idA  B id=$idB"

Write-Host "=== LOGIN / REFRESH / ME ==="
$login = Call -method POST -path "/auth/login" -body @{email="alice@example.com"; password="password123"}
$tokenA = $login.data.access_token
Call -method POST -path "/auth/refresh" -body @{refresh_token=$login.data.refresh_token}
Call -method GET -path "/auth/me" -token $tokenA

Write-Host "=== PROFILE / SETTINGS ==="
Call -method PUT -path "/user/profile" -token $tokenA -body @{display_name="Alice A"; username="alice"}
Call -method GET -path "/user/profile" -token $tokenA
Call -method PUT -path "/user/settings" -token $tokenA -body @{theme="dark"; language="ru"; notifications_enabled=$true}
Call -method GET -path "/user/settings" -token $tokenA

Write-Host "=== USER SEARCH ==="
Call -method GET -path "/user/search?q=al" -token $tokenA

Write-Host "=== PRIVATE CHAT ==="
$pc = Call -method POST -path "/chats/private" -token $tokenA -body @{user_id=$idB}
$pcid = $pc.data.id
Write-Host "  private chat=$pcid"

Write-Host "=== GROUP CHAT + ADMIN ==="
$gc = Call -method POST -path "/chats/group" -token $tokenA -body @{title="Gang"; participant_ids=@($idB)}
$gcid = $gc.data.id
Write-Host "  group chat=$gcid"
Call -method GET -path "/chats" -token $tokenA
# GET single chat not implemented

Call -method GET -path "/chats/$gcid/participants" -token $tokenA
Call -method POST -path "/chats/$gcid/participants/$idB/promote" -token $tokenA
Call -method PUT -path "/chats/$gcid" -token $tokenB -body @{title="Gang Renamed by Admin"}
$c = Call -method POST -path "/auth/register" -body @{email="carol@example.com"; username="carol"; password="password123"}
$idC = $c.data.user.id
Call -method POST -path "/chats/$gcid/participants" -token $tokenB -body @{user_id=$idC}
Call -method POST -path "/chats/$gcid/participants/$idB/demote" -token $tokenA
Call -method PUT -path "/chats/$gcid" -token $tokenB -body @{title="Bob no longer admin"}
Call -method DELETE -path "/chats/$gcid/participants/$idC" -token $tokenA

Write-Host "=== MESSAGES ==="
$m1 = Call -method POST -path "/messages" -token $tokenA -body @{chat_id=$gcid; type="text"; text="Hello gang"}
$m1id = $m1.data.id
Call -method POST -path "/messages" -token $tokenB -body @{chat_id=$gcid; type="text"; text="Hi Alice"; reply_to=$m1id}
Call -method GET -path "/chats/$gcid/messages" -token $tokenA
Call -method PUT -path "/messages/$m1id" -token $tokenA -body @{text="Hello gang (edited)"}
Call -method GET -path "/chats/$gcid/unread" -token $tokenB
Call -method POST -path "/chats/$gcid/read" -token $tokenB -body @{message_ids=@($m1id)}
Call -method GET -path "/chats/$gcid/unread" -token $tokenB

Write-Host "=== REACTIONS ==="
Call -method POST -path "/messages/$m1id/reactions" -token $tokenA -body @{emoji="like"}
Call -method POST -path "/messages/$m1id/reactions" -token $tokenB -body @{emoji="like"}
Call -method GET -path "/messages/$m1id/reactions" -token $tokenA
Call -method DELETE -path "/messages/$m1id/reactions?emoji=like" -token $tokenA

Write-Host "=== PIN / FORWARD / SEARCH ==="
Call -method POST -path "/chats/$gcid/pin/$m1id" -token $tokenA
Call -method DELETE -path "/chats/$gcid/pin/$m1id" -token $tokenA
$pc2 = Call -method POST -path "/chats/private" -token $tokenA -body @{user_id=$idC}
Call -method POST -path "/messages/$m1id/forward" -token $tokenA -body @{chat_id=$pc2.data.id}
Call -method GET -path "/chats/$gcid/messages/search?q=edited" -token $tokenA
Call -method GET -path "/messages/search?q=Hi" -token $tokenA

Write-Host "=== TYPING ==="
Call -method POST -path "/chats/$gcid/typing" -token $tokenA

Write-Host "=== ADMIN DELETE FOR ALL ==="
Call -method DELETE -path "/chats/$gcid/messages/$m1id/admin" -token $tokenA

Write-Host "=== BLOCKS ==="
Call -method POST -path "/user/block" -token $tokenA -body @{user_id=$idB}
Call -method GET -path "/user/blocks" -token $tokenA
Call -method DELETE -path "/user/block/$idB" -token $tokenA

Write-Host "=== MEDIA UPLOAD (curl) ==="
$pngPath = "$env:TEMP\px.png"
[byte[]]$px = @(0x89,0x50,0x4E,0x47,0x0D,0x0A,0x1A,0x0A,0x00,0x00,0x00,0x0D,0x49,0x48,0x44,0x52,0x00,0x00,0x00,0x01,0x00,0x00,0x00,0x01,0x08,0x06,0x00,0x00,0x00,0x1F,0x15,0xC4,0x89,0x00,0x00,0x00,0x0A,0x49,0x44,0x41,0x54,0x78,0x9C,0x63,0x00,0x01,0x00,0x00,0x05,0x00,0x01,0x0D,0x0A,0x2D,0xB4,0x00,0x00,0x00,0x00,0x49,0x45,0x4E,0x44,0xAE,0x42,0x60,0x82)
[System.IO.File]::WriteAllBytes($pngPath, $px)
curl.exe -s -X POST "$base/media/upload" -H "Authorization: Bearer $tokenA" -F "type=image" -F "file=@$pngPath;type=image/png"
Write-Host ""

Write-Host "=== DEVICES / NOTIFICATIONS ==="
Call -method POST -path "/notifications/device" -token $tokenA -body @{platform="android"; token="test-token-123"}
Call -method GET -path "/notifications" -token $tokenB
Call -method POST -path "/notifications/read" -token $tokenB -body @{}

Write-Host "=== CALLS ==="
$call = Call -method POST -path "/calls" -token $tokenA -body @{chat_id=$gcid; type="audio"}
$callid = $call.data.id
# GET single call not implemented
Call -method POST -path "/calls/$callid/end" -token $tokenA

Write-Host "=== LOGOUT ==="
Call -method POST -path "/auth/logout" -token $tokenA

Write-Host "=== DONE ==="
