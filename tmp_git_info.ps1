git -C "C:\Code\AI\云雀\yunque-agent" log --reverse --all --format="%H %ci" | Select-Object -First 3
Write-Output "---"
git -C "C:\Code\AI\云雀\yunque-agent" shortlog -sn --all
Write-Output "---"
git -C "C:\Code\AI\云雀\yunque-agent" log --all --oneline | Measure-Object | Select-Object Count
