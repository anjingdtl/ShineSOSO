' Stops only EasySearch and Prowlarr processes launched from this portable folder.
Option Explicit
Dim fso, shell, root, service, process, command, rootLower, stopped, portFile
Set fso = CreateObject("Scripting.FileSystemObject")
Set shell = CreateObject("Wscript.Shell")
root = fso.GetParentFolderName(WScript.ScriptFullName)
rootLower = LCase(root)
stopped = False
Set service = GetObject("winmgmts:\\.\root\cimv2")

For Each process In service.ExecQuery("Select * From Win32_Process Where Name='easysearch.exe' Or Name='Prowlarr.exe'")
  command = LCase(process.CommandLine)
  If InStr(command, rootLower) > 0 Then
    process.Terminate
    stopped = True
  End If
Next

' The normal portable location uses this port file. Removing it after a
' forced stop prevents a stale lock marker from confusing the next launch.
If stopped Then
  portFile = fso.BuildPath(shell.ExpandEnvironmentStrings("%APPDATA%"), "EasySearch\data\.port")
  If fso.FileExists(portFile) Then fso.DeleteFile portFile, True
End If
