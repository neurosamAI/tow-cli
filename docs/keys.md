# 키 관리

이 문서는 tow-cli 저장소가 쓰는 키의 목록이다. 키 값은 여기에 적지 않으며, 값은 사내 시크릿 보관소에 있다. 아래 키는 모두 GitHub Actions 저장소 시크릿으로 등록해 릴리스 워크플로(`.github/workflows/release.yml`, `v*` 태그 push 때 실행)에서만 쓴다. 키를 새로 만들거나 바꾸면 이 문서와 사내 보관소의 사본을 함께 고친다.

## 저장소 시크릿

| 이름 | 용도 | 쓰는 곳 | 잃었을 때 |
|---|---|---|---|
| NPM_TOKEN | npm 패키지 배포 토큰 | `release.yml` `npm` 잡(`packaging/npm`, `NODE_AUTH_TOKEN` 으로 넘김) | npm 에서 배포 권한 토큰을 다시 발급해 저장소 시크릿을 바꾼다 |
| TAP_GITHUB_TOKEN | Homebrew tap 저장소(`neurosamAI/homebrew-tap`)에 repository dispatch 를 보내는 GitHub 토큰 | `release.yml` `homebrew` 잡 | 그 저장소에 dispatch 할 수 있는 권한으로 GitHub 토큰을 다시 발급한다 |
| VSCE_PAT | VS Code Marketplace 확장 배포 토큰 | `release.yml` `vscode` 잡(`integrations/vscode`) | Azure DevOps 에서 Marketplace 배포 권한으로 PAT 를 다시 발급한다 |
