package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"agent-platform/pkg/browseragent"
)

func main() {
	// ============================================================
	// 解析 CSDN Cookie
	// ============================================================
	cookieStr := `uuid_tt_dd=10_28834251810-1769130812137-887984; fid=20_60604000468-1769130811035-867028; UserName=m0_54140879; UserInfo=818a4e24e3e94ee686ebc30893c5cd37; UserToken=818a4e24e3e94ee686ebc30893c5cd37; UserNick=%40atweiwei; AU=81C; UN=m0_54140879; BT=1771983953314; p_uid=U010000; _ga_7W1N0GEY1P=GS2.1.s1772271497$o2$g1$t1772271553$j4$l0$h0; c_segment=0; dc_sid=af1d1f0a2d3edda642b102f162585819; HMACCOUNT=E7EDBA4BACF8DB33; csdn_newcert_m0_54140879=1; __gads=ID=d5be9838ff3960d1:T=1771986096:RT=1780467507:S=ALNI_MZFQRXkZ5I6fcH0PELWmbX_bsrG7w; __gpi=UID=0000120a12fddfd5:T=1771986096:RT=1780467507:S=ALNI_MbVWdocEUf57SA5GtRJ_wmLlMItqg; __eoi=ID=92fb3c4de9d6434e:T=1771986096:RT=1780467507:S=AA-AfjZS8fXa6MVXnHHCEUSXmIbJ; FCCDCF=%5Bnull%2Cnull%2Cnull%2Cnull%2Cnull%2Cnull%2C%5B%5B32%2C%22%5B%5C%224ce2af19-5910-4faf-a38d-99ed5917ee42%5C%22%2C%5B1771986091%2C184000000%5D%5D%22%5D%5D%5D; FCNEC=%5B%5B%22AKsRol9XN9CFq0bn3WYYhS5vR8hAmNmBMJh0zBmU-iTDiDUpZc-lkvMUmncJeWtEGlRvFUyFg_RFLpR565BKriD3Od0coLS8gtRiHPBLJwGezzrT3PKay5uoi8BfDkXgEDueHC-RttRS02K86_NoWvNcFD_bcS74Tg%3D%3D%22%5D%5D; c_first_ref=www.google.com; c_first_page=https%3A//www.csdn.net/; Hm_lvt_6bcd52f51e9b3dce32bec4a3997715ac=1778728059,1779934459; c_ab_test=1; is_advert=1; _clck=17qlwmt%5E2%5Eg6s%5E0%5E2214; vip_auto_popup=1; _clsk=1ldv8ao%5E1781054449915%5E2%5E0%5Ex.clarity.ms%2Fcollect; dc_session_id=10_1781081559940.885022; c_dsid=11_1781081539976.811764; c-sidebar-collapse=0; creative_popup=%7B%22arrowIcon%22%3A%22https%3A//i-operation.csdnimg.cn/images/394e99a49b19451fb89baacbe7ae5f0e.png%22%2C%22img%22%3A%22https%3A//i-operation.csdnimg.cn/images/1e8f150a68a74c53a83400d69f535a92.png%22%2C%22imgStyle%22%3A%22height%3A%2074px%3B%22%2C%22darkCfg%22%3A%7B%7D%2C%22role%22%3A%22write%22%2C%22report%22%3A%7B%22spm%22%3A%223001.11121%22%2C%22extra%22%3A%22%22%7D%2C%22style%22%3A%22%22%2C%22arrowIconStyle%22%3A%22%22%2C%22url%22%3A%22https%3A//mall.csdn.net/vip%3Futm_source%3D260618_vip_blogrighticon%22%2C%22newTab%22%3Afalse%2C%22userName%22%3A%22m0_54140879%22%7D; creative_btn_mp=3; _ga=GA1.2.556726872.1769130815; _gid=GA1.2.1815836052.1781081542; _gat_gtag_UA_127895514_1=1; c_utm_medium=distribute.pc_search_hot_word.none-task-hot_word-alirecmd-2-tikio-null-null.172%5Ev8%5Econtrol; fe_request_id=1781081547160_6803_0689471; c_pref=https%3A//so.csdn.net/so/search%3Fspm%3D1000.2115.3001.7499%26q%3Dtikio%26t%3D%26u%3D%26utm_medium%3Ddistribute.pc_search_hot_word.none-task-hot_word-alirecmd-2-tikio-null-null.172%255Ev8%255Econtrol%26depth_1-utm_source%3Ddistribute.pc_search_hot_word.none-task-hot_word-alirecmd-2-tikio-null-null.172%255Ev8%255Econtrol; c_ref=https%3A//www.csdn.net/; c_utm_source=cknow_pc_ntoolbar; utm_source=cknow_pc_ntoolbar; log_Id_click=6; _ga_JJBD2VG1H7=GS2.1.s1781081541$o4$g1$t1781081569$j32$l0$h0; c_page_id=default; dc_tos=tgerc1; log_Id_pv=5; log_Id_view=117; Hm_lpvt_6bcd52f51e9b3dce32bec4a3997715ac=1781081570`

	// 解析 Cookie 字符串
	cookies := parseCookies(cookieStr, ".csdn.net")

	fmt.Printf("解析出 %d 个 Cookie\n", len(cookies))
	for _, c := range cookies {
		fmt.Printf("  - %s: %s\n", c.Name, c.Value[:min(20, len(c.Value))]+"...")
	}

	// ============================================================
	// 配置 LLM Client
	// ============================================================
	apiKey := "sk-6eb65fcf5d17491ca10b984efe1f43e7"
	baseURL := "https://dashscope.aliyuncs.com/compatible-mode/v1"
	model := "qwen-plus"

	llmClient := browseragent.NewOpenAIClient(apiKey, baseURL, model)

	// ============================================================
	// 创建 Browser Agent
	// ============================================================
	fmt.Println("\n=== 创建 Browser Agent ===")

	// 使用内存存储（测试用）
	cookieStorage := browseragent.NewMemoryCookieStorage()

	agent := browseragent.New(llmClient, browseragent.NewBrowser(),
		browseragent.WithMaxSteps(50), // 增加步数，浏览器操作需要更多步骤
		browseragent.WithDebug(true),
		browseragent.WithCookies(cookies),                    // 注入 Cookie
		browseragent.WithCookieStorage(cookieStorage),        // Cookie 存储
		browseragent.WithUserContext("test-user", "default"), // 用户上下文
		browseragent.WithAutoSaveCookie(true),                // 自动保存
	)

	// ============================================================
	// 执行任务：访问 CSDN 博客，查看文章列表
	// ============================================================
	fmt.Println("\n=== 执行任务：访问 CSDN 博客 ===")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute) // 增加到 10 分钟
	defer cancel()

	result, err := agent.RunWithDomain(ctx,
		"打开 https://blog.csdn.net/m0_54140879 的主页，查看我发表的文章列表，告诉我文章标题",
		".csdn.net",
	)

	if err != nil {
		fmt.Printf("❌ 错误: %v\n", err)
		return
	}

	fmt.Printf("\n=== 结果 ===\n")
	fmt.Printf("成功: %v\n", result.Success)
	fmt.Printf("步数: %d\n", result.Steps)
	fmt.Printf("耗时: %v\n", result.Duration)
	fmt.Printf("回答: %s\n", result.Answer)

	// 打印执行历史
	fmt.Printf("\n=== 执行历史 ===\n")
	for _, step := range result.StepHistory {
		fmt.Printf("Step %d: %s -> %s\n", step.Step, step.Action.Type, step.Result)
	}

	// ============================================================
	// 检查保存的 Cookie
	// ============================================================
	fmt.Printf("\n=== 检查保存的 Cookie ===\n")

	storedCookies, err := cookieStorage.GetAll(context.Background(), "test-user", "default")
	if err != nil {
		fmt.Printf("获取 Cookie 失败: %v\n", err)
	} else {
		fmt.Printf("已保存 %d 个 Cookie\n", len(storedCookies))
		for _, c := range storedCookies {
			fmt.Printf("  - %s (%s)\n", c.Name, c.Domain)
		}
	}
}

// parseCookies 解析 Cookie 字符串
func parseCookies(cookieStr, domain string) []browseragent.Cookie {
	var cookies []browseragent.Cookie

	parts := strings.Split(cookieStr, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}

		name := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])

		cookies = append(cookies, browseragent.Cookie{
			Name:   name,
			Value:  value,
			Domain: domain,
		})
	}

	return cookies
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}