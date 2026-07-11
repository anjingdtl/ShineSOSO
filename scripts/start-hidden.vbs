' start-hidden.vbs - silently start dist\easysearch.exe without flashing a console.
' Usage: double-click in Explorer, or run via
'   cscript //nologo scripts\start-hidden.vbs

Option Explicit

Const WIN_HIDDEN = 0      ' WshShell.Run WindowStyle: hidden
Const NON_BLOCK  = False ' do not block the script waiting for the child

Dim fso, shell, repoRoot, exePath, dataDir, logPath
Set fso   = CreateObject("Scripting.FileSystemObject")
Set shell = CreateObject("Wscript.Shell")

repoRoot = fso.GetParentFolderName(fso.GetParentFolderName(WScript.ScriptFullName))
exePath  = fso.BuildPath(repoRoot, "dist\easysearch.exe")
dataDir  = fso.BuildPath(shell.ExpandEnvironmentStrings("%APPDATA%"), "EasySearch\data")
logPath  = fso.BuildPath(dataDir, "start.log")

If Not fso.FileExists(exePath) Then
    Call Log("missing exe: " & exePath)
    MsgBox _
        "Missing " & vbCrLf & exePath & vbCrLf & vbCrLf & _
        "Please run scripts\build.ps1 first to build.", _
        vbCritical, "EasySearch start failed"
    WScript.Quit 1
End If

EnsureFolder dataDir

On Error Resume Next
shell.Run """" + exePath + """", WIN_HIDDEN, NON_BLOCK
If Err.Number <> 0 Then
    Dim msg
    msg = "Start failed: " & vbCrLf & exePath & vbCrLf & vbCrLf & _
          "Error: " & Err.Description
    Call Log("start failed: " & Err.Description)
    MsgBox msg, vbCritical, "EasySearch start failed"
    WScript.Quit 2
End If
On Error Goto 0

Call Log("started " & exePath)

' ---- helpers ----
Sub Log(line)
    On Error Resume Next
    Dim stream
    EnsureFolder fso.GetParentFolderName(logPath)
    Set stream = fso.OpenTextFile(logPath, 8, True) ' ForAppending
    If Err.Number = 0 Then
        stream.WriteLine FormatDateTime(Now, vbGeneralDate) & "  " & line
        stream.Close
    End If
    On Error Goto 0
End Sub

' Create a directory and any missing parents. FileSystemObject.CreateFolder
' only creates one level, so a first launch would otherwise fail when
' %APPDATA%\EasySearch does not yet exist.
Sub EnsureFolder(path)
    On Error Resume Next
    If fso.FolderExists(path) Then Exit Sub
    Dim parent
    parent = fso.GetParentFolderName(path)
    If parent <> "" And Not fso.FolderExists(parent) Then EnsureFolder parent
    If Not fso.FolderExists(path) Then fso.CreateFolder path
    On Error Goto 0
End Sub
