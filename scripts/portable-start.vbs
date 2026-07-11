' Portable launcher: kept next to easysearch.exe inside the release ZIP.
Option Explicit
Const WIN_HIDDEN = 0
Dim fso, shell, root, exePath
Set fso = CreateObject("Scripting.FileSystemObject")
Set shell = CreateObject("Wscript.Shell")
root = fso.GetParentFolderName(WScript.ScriptFullName)
exePath = fso.BuildPath(root, "easysearch.exe")
If Not fso.FileExists(exePath) Then
  MsgBox "找不到 easysearch.exe。请重新解压完整发布包。", vbCritical, "EasySearch"
  WScript.Quit 1
End If
shell.Run """" & exePath & """", WIN_HIDDEN, False
