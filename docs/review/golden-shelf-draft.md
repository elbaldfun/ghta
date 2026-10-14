# golden 货架标注草稿（待人审，第 2 版）

来源：2026-10-14 补了货架说明和判别规则之后的判定结果（golden 316 条，每条一遍短答，所以个别条目有单遍的偶然误差）。第 1 版里硬塞货架的错例（rufus 放手机工具、superset 放监控、caddy 放代理、编码智能体放终端）已在提示词里修掉。

共 59 条：与现有标注不同 8，新增 23，一致 28。开发者货架且 golden 领域已有同一叶子的不再列出，那些不需要新标注。

怎么审：在"确认"列填最终货架，或填 `excluded`。"备注"是 Claude 的看法。

| 状态 | 仓库 | 形态 | golden 现有货架 | 模型货架 | 备注 | 确认 |
|---|---|---|---|---|---|---|
| 与现有标注不同 | SigNoz/signoz | app | selfhosted/monitoring | infra/monitoring | 面向运维的可观测平台，建议改 infra/monitoring |  |
| 与现有标注不同 | calcom/cal.diy | app | productivity/todo, selfhosted/cloud | excluded | golden 的 productivity/todo 对，模型错 |  |
| 与现有标注不同 | go-gitea/gitea | app | selfhosted/cloud | devtools/vcs | golden 的 selfhosted/cloud 是网盘与相册，建议改 devtools/vcs |  |
| 与现有标注不同 | gogs/gogs | app | selfhosted/cloud | excluded | 同 gitea，建议 devtools/vcs |  |
| 与现有标注不同 | iOfficeAI/OfficeCLI | cli | productivity/docs | excluded | golden 的 productivity/docs 对，模型错 |  |
| 与现有标注不同 | santifer/career-ops | cli | productivity/notes | excluded | 我不确定，请你定 |  |
| 与现有标注不同 | searxng/searxng | app | selfhosted/cloud | excluded | 没有合适货架，建议 excluded |  |
| 与现有标注不同 | siyuan-note/siyuan | app | productivity/notes, selfhosted/cloud | productivity/knowledge | notes 和 knowledge 都说得通，保留 golden |  |
| 新增 | AUTOMATIC1111/stable-diffusion-webui | app | — | ai/generation |  |  |
| 新增 | Anuken/Mindustry | app | — | games/games |  |  |
| 新增 | Comfy-Org/ComfyUI | app | — | ai/generation |  |  |
| 新增 | ItzCrazyKns/Vane | app | — | ai/assistant |  |  |
| 新增 | NousResearch/hermes-agent | cli | — | agents/coding |  |  |
| 新增 | PixelGuys/Cubyz | app | — | games/games |  |  |
| 新增 | clash-verge-rev/clash-verge-rev | app | — | network/proxy |  |  |
| 新增 | fatedier/frp | software | — | selfhosted/tunnel |  |  |
| 新增 | hydralauncher/hydra | app | — | games/games |  |  |
| 新增 | ish-app/ish | app | — | system/phone |  |  |
| 新增 | jamiepine/voicebox | app | — | creative/audio |  |  |
| 新增 | lllyasviel/Fooocus | app | — | ai/generation |  |  |
| 新增 | lobehub/lobe-chat | app | — | ai/assistant |  |  |
| 新增 | nullclaw/nullclaw | software | — | ai/assistant |  |  |
| 新增 | ogulcancelik/herdr | cli | — | devtools/terminal |  |  |
| 新增 | ollama/ollama | cli | — | ai/local-llm |  |  |
| 新增 | openclaw/openclaw | app | — | agents/coding | 是个人 AI 助手，建议 ai/assistant，模型这一遍错 |  |
| 新增 | pbatard/rufus | app | — | system/files | 启动盘工具，没有合适货架，建议 excluded |  |
| 新增 | projectdiscovery/subfinder | cli | — | security/offensive |  |  |
| 新增 | rtk-ai/rtk | cli | — | devtools/terminal |  |  |
| 新增 | schollz/croc | cli | — | system/transfer |  |  |
| 新增 | shadps4-emu/shadPS4 | app | — | games/emulator |  |  |
| 新增 | sherlock-project/sherlock | cli | — | security/offensive |  |  |
| 一致 | FFmpeg/FFmpeg | cli | creative/video | creative/video |  |  |
| 一致 | Genymobile/scrcpy | cli | system/phone | system/phone |  |  |
| 一致 | LadybirdBrowser/ladybird | app | network/browser | network/browser |  |  |
| 一致 | OpenCut-app/OpenCut | app | creative/video | creative/video |  |  |
| 一致 | Stirling-Tools/Stirling-PDF | app | productivity/docs | productivity/docs |  |  |
| 一致 | bitwarden/clients | app | security/passwords | security/passwords |  |  |
| 一致 | coollabsio/coolify | app | selfhosted/cloud | selfhosted/cloud | 自托管 PaaS，不是网盘；建议 excluded 或 infra/containers |  |
| 一致 | excalidraw/excalidraw | app | creative/whiteboard | creative/whiteboard |  |  |
| 一致 | gorhill/uBlock | app | network/browser-ext | network/browser-ext |  |  |
| 一致 | immich-app/immich | app | selfhosted/cloud | selfhosted/cloud |  |  |
| 一致 | janhq/jan | app | ai/local-llm | ai/local-llm |  |  |
| 一致 | langgenius/dify | app | ai/builder | ai/builder |  |  |
| 一致 | logseq/logseq | app | productivity/notes | productivity/notes |  |  |
| 一致 | louislam/uptime-kuma | app | selfhosted/monitoring | selfhosted/monitoring |  |  |
| 一致 | makeplane/plane | app | productivity/todo, selfhosted/cloud | productivity/todo |  |  |
| 一致 | microsoft/PowerToys | app | system/launcher | system/launcher |  |  |
| 一致 | mpv-player/mpv | app | media/player | media/player |  |  |
| 一致 | n8n-io/n8n | app | selfhosted/automation | selfhosted/automation |  |  |
| 一致 | nomic-ai/gpt4all | app | ai/local-llm | ai/local-llm |  |  |
| 一致 | oblien/openship | app | selfhosted/cloud | selfhosted/cloud | 我不确定，请你定 |  |
| 一致 | obsproject/obs-studio | app | system/screenshot | system/screenshot |  |  |
| 一致 | open-webui/open-webui | app | ai/assistant | ai/assistant |  |  |
| 一致 | refined-github/refined-github | app | network/browser-ext | network/browser-ext |  |  |
| 一致 | rustdesk/rustdesk | app | network/remote | network/remote |  |  |
| 一致 | syncthing/syncthing | app | system/transfer | system/transfer |  |  |
| 一致 | usememos/memos | app | selfhosted/cloud, productivity/notes | productivity/notes |  |  |
| 一致 | yt-dlp/yt-dlp | cli | network/download | network/download |  |  |
| 一致 | ytdl-org/youtube-dl | cli | network/download | network/download |  |  |
