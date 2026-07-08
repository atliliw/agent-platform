-- ==================== Rules ====================
INSERT OR IGNORE INTO rules (id, agent_id, name, type, config, enabled, tenant_id, created_at, updated_at) VALUES
('rule-001', 'agent-chat', '内容安全过滤', 'content_filter', '{"blocked_keywords":["暴力","色情"],"action":"block"}', 1, 'default', '2026-07-01 10:00:00', '2026-07-05 10:00:00'),
('rule-002', 'agent-rag', '响应长度限制', 'response_limit', '{"max_tokens":4096,"action":"truncate"}', 1, 'default', '2026-07-01 11:00:00', '2026-07-05 11:00:00'),
('rule-003', 'agent-code', '代码安全审查', 'code_review', '{"check_sql_injection":true,"check_xss":true,"action":"warn"}', 1, 'default', '2026-07-02 09:00:00', '2026-07-05 09:00:00'),
('rule-004', 'agent-chat', '速率限制', 'rate_limit', '{"max_requests_per_minute":60,"action":"queue"}', 1, 'default', '2026-07-02 14:00:00', '2026-07-05 14:00:00'),
('rule-005', 'agent-rag', '敏感信息脱敏', 'pii_filter', '{"patterns":["phone","email","id_card"],"action":"mask"}', 1, 'default', '2026-07-03 08:00:00', '2026-07-05 08:00:00');

-- ==================== SLOs (repo model) ====================
INSERT OR IGNORE INTO slos (id, agent_id, name, target, type, tenant_id, created_at) VALUES
('slo-001', 'agent-chat', 'Chat响应延迟', 0.95, 'latency', 'default', '2026-07-01 10:00:00'),
('slo-002', 'agent-rag', 'RAG准确率', 0.90, 'accuracy', 'default', '2026-07-01 10:00:00'),
('slo-003', 'agent-code', '代码生成成功率', 0.85, 'success_rate', 'default', '2026-07-01 10:00:00');

-- ==================== SLO Definitions (engine model) ====================
INSERT OR IGNORE INTO slo_definitions (id, agent_id, name, type, target, window, alert_threshold, burn_rate_alert, tenant_id, created_at, updated_at) VALUES
('slo-def-001', 'agent-chat', 'Chat响应延迟P99<2s', 'latency', 0.95, '7d', 0.8, 1, 'default', '2026-07-01 10:00:00', '2026-07-05 10:00:00'),
('slo-def-002', 'agent-rag', 'RAG检索准确率>90%', 'accuracy', 0.90, '30d', 0.85, 1, 'default', '2026-07-01 10:00:00', '2026-07-05 10:00:00'),
('slo-def-003', 'agent-code', '代码生成成功率>85%', 'success_rate', 0.85, '7d', 0.75, 1, 'default', '2026-07-01 10:00:00', '2026-07-05 10:00:00'),
('slo-def-004', 'agent-chat', 'Chat可用性>99.5%', 'availability', 0.995, '30d', 0.99, 1, 'default', '2026-07-02 10:00:00', '2026-07-05 10:00:00');

-- ==================== SLO Events ====================
INSERT OR IGNORE INTO slo_events (id, slo_id, agent_id, event_type, value, labels, timestamp) VALUES
('slo-evt-001', 'slo-def-001', 'agent-chat', 'success', 1200.0, '{"endpoint":"/chat"}', '2026-07-05 08:00:00'),
('slo-evt-002', 'slo-def-001', 'agent-chat', 'success', 980.0, '{"endpoint":"/chat"}', '2026-07-05 08:05:00'),
('slo-evt-003', 'slo-def-001', 'agent-chat', 'failure', 3500.0, '{"endpoint":"/chat"}', '2026-07-05 08:10:00'),
('slo-evt-004', 'slo-def-002', 'agent-rag', 'success', 0.92, '{"query_type":"factual"}', '2026-07-05 09:00:00'),
('slo-evt-005', 'slo-def-002', 'agent-rag', 'success', 0.88, '{"query_type":"analytical"}', '2026-07-05 09:10:00'),
('slo-evt-006', 'slo-def-003', 'agent-code', 'success', 1.0, '{"language":"python"}', '2026-07-05 10:00:00'),
('slo-evt-007', 'slo-def-003', 'agent-code', 'failure', 0.0, '{"language":"rust"}', '2026-07-05 10:15:00'),
('slo-evt-008', 'slo-def-004', 'agent-chat', 'success', 200.0, '{"endpoint":"/chat"}', '2026-07-05 11:00:00'),
('slo-evt-009', 'slo-def-001', 'agent-chat', 'success', 1500.0, '{"endpoint":"/chat"}', '2026-07-05 11:05:00'),
('slo-evt-010', 'slo-def-002', 'agent-rag', 'success', 0.95, '{"query_type":"factual"}', '2026-07-05 11:10:00');

-- ==================== AB Tests ====================
INSERT OR IGNORE INTO ab_tests (id, name, control_model, variant_model, traffic_split, agent_id, tenant_id, status, type, control_config, variant_config, created_at) VALUES
('abtest-001', 'Chat模型对比实验', 'gpt-4o', 'claude-3.5-sonnet', 0.5, 'agent-chat', 'default', 'running', 'model', '{"temperature":0.7}', '{"temperature":0.7}', '2026-07-01 10:00:00'),
('abtest-002', 'RAG检索策略对比', 'hybrid_search', 'semantic_search', 0.3, 'agent-rag', 'default', 'completed', 'prompt', '{"top_k":5,"rerank":true}', '{"top_k":10,"rerank":false}', '2026-06-25 10:00:00'),
('abtest-003', '代码生成Prompt优化', 'v1_baseline', 'v2_cot', 0.5, 'agent-code', 'default', 'running', 'prompt', '{"prompt":"Generate code"}', '{"prompt":"Think step by step then generate code"}', '2026-07-03 14:00:00');

-- ==================== AB Test Experiment Results ====================
INSERT OR IGNORE INTO experiment_results (id, experiment_id, variant, metric_name, metric_value, sample_size, confidence, created_at) VALUES
('expr-001', 'abtest-001', 'control', 'satisfaction', 4.2, 150, 0.92, '2026-07-05 10:00:00'),
('expr-002', 'abtest-001', 'variant', 'satisfaction', 4.5, 148, 0.89, '2026-07-05 10:00:00'),
('expr-003', 'abtest-001', 'control', 'latency_ms', 1200.0, 150, 0.95, '2026-07-05 10:00:00'),
('expr-004', 'abtest-001', 'variant', 'latency_ms', 980.0, 148, 0.93, '2026-07-05 10:00:00');

-- ==================== Rollback Configs ====================
INSERT OR IGNORE INTO rollback_configs (id, agent_id, name, config_type, target_id, max_snapshots, cool_down_period, auto_rollback, rollback_on_slo, slo_threshold, tenant_id, created_at, updated_at) VALUES
('rb-cfg-001', 'agent-chat', 'Chat配置回滚', 'agent_config', 'agent-chat-config', 10, 30, 1, 1, 0.95, 'default', '2026-07-01 10:00:00', '2026-07-05 10:00:00'),
('rb-cfg-002', 'agent-rag', 'RAG模型回滚', 'model_config', 'rag-model-config', 5, 60, 1, 1, 0.90, 'default', '2026-07-01 11:00:00', '2026-07-05 11:00:00'),
('rb-cfg-003', 'agent-code', '代码审查规则回滚', 'rule_config', 'code-review-rules', 8, 15, 0, 0, 0.0, 'default', '2026-07-02 09:00:00', '2026-07-05 09:00:00');

-- ==================== Config Snapshots ====================
INSERT OR IGNORE INTO config_snapshots (id, config_id, snapshot_data, version, description, created_at, created_by, is_active) VALUES
('snap-001', 'rb-cfg-001', '{"model":"gpt-4o","temperature":0.7,"max_tokens":4096}', 'v1.0', '初始配置', '2026-07-01 10:00:00', 'admin', 0),
('snap-002', 'rb-cfg-001', '{"model":"gpt-4o","temperature":0.5,"max_tokens":8192}', 'v1.1', '降低温度增加tokens', '2026-07-03 14:00:00', 'admin', 1),
('snap-003', 'rb-cfg-002', '{"model":"text-embedding-3-large","top_k":5}', 'v1.0', 'RAG初始配置', '2026-07-01 11:00:00', 'admin', 1);

-- ==================== Rollback Events ====================
INSERT OR IGNORE INTO rollback_events (id, config_id, snapshot_id, event_type, triggered_by, from_version, to_version, success, error, duration_ms, timestamp) VALUES
('rb-evt-001', 'rb-cfg-001', 'snap-001', 'rollback', 'auto', 'v1.1', 'v1.0', 1, '', 350, '2026-07-04 03:20:00'),
('rb-evt-002', 'rb-cfg-001', 'snap-002', 'restore', 'manual', 'v1.0', 'v1.1', 1, '', 200, '2026-07-04 10:00:00');

-- ==================== Change Events (RCA) ====================
INSERT OR IGNORE INTO change_events (id, agent_id, change_type, resource_id, resource_type, description, old_value, new_value, timestamp, user, source, metadata, tenant_id) VALUES
('chg-001', 'agent-chat', 'config', 'chat-temperature', 'agent_config', '调整Chat温度参数', '0.7', '0.5', '2026-07-03 14:00:00', 'admin', 'manual', '{"reason":"用户反馈回答太随机"}', 'default'),
('chg-002', 'agent-rag', 'model', 'rag-embedding', 'model_config', '更换Embedding模型', 'text-embedding-ada-002', 'text-embedding-3-large', '2026-07-02 09:00:00', 'admin', 'manual', '{"reason":"提升检索质量"}', 'default'),
('chg-003', 'agent-code', 'deployment', 'code-agent-v2', 'deployment', '代码Agent升级到v2', 'v1.5.0', 'v2.0.0', '2026-07-04 08:00:00', 'ci-pipeline', 'auto', '{"commit":"abc123"}', 'default'),
('chg-004', 'agent-chat', 'feature_flag', 'stream-mode', 'feature_flag', '启用流式输出', 'false', 'true', '2026-07-04 10:00:00', 'admin', 'manual', '{}', 'default'),
('chg-005', 'agent-rag', 'rule', 'rag-content-filter', 'rule_config', '更新内容过滤规则', 'v1', 'v2', '2026-07-05 08:00:00', 'admin', 'manual', '{"added_patterns":["financial_advice"]}', 'default');

-- ==================== Incident Events (RCA) ====================
INSERT OR IGNORE INTO incident_events (id, agent_id, title, description, severity, impact, detected_at, resolved_at, status, metadata, tenant_id) VALUES
('inc-001', 'agent-chat', 'Chat响应延迟飙升', 'P99延迟从1.2s升至3.5s，影响30%用户', 'high', '30%用户体验下降', '2026-07-04 03:15:00', '2026-07-04 03:45:00', 'resolved', '{"p99_ms":3500,"affected_users":300}', 'default'),
('inc-002', 'agent-rag', 'RAG检索准确率下降', '检索准确率从92%降至78%', 'medium', '检索质量下降', '2026-07-03 16:00:00', '2026-07-03 17:30:00', 'resolved', '{"accuracy_drop":0.14}', 'default'),
('inc-003', 'agent-code', '代码生成服务不可用', '代码生成服务连续5分钟返回500错误', 'critical', '服务完全不可用', '2026-07-05 02:00:00', NULL, 'active', '{"error_rate":1.0,"duration_min":5}', 'default');

-- ==================== Chaos Experiments (repo model) ====================
INSERT OR IGNORE INTO chaos_experiments (id, name, description, agent_id, fault_type, fault_config, duration, blast_radius, auto_stop_on_slo, slo_threshold, status, created_at, updated_at, started_at, ended_at, tenant_id) VALUES
('chaos-001', 'Chat延迟注入测试', '模拟网络延迟对Chat服务的影响', 'agent-chat', 'latency', '{"latency_ms":2000,"jitter":500}', 10, 0.2, 1, 0.95, 'completed', '2026-07-02 10:00:00', '2026-07-02 10:15:00', '2026-07-02 10:00:00', '2026-07-02 10:10:00', 'default'),
('chaos-002', 'RAG服务降级测试', '模拟RAG服务部分节点故障', 'agent-rag', 'service_degradation', '{"degrade_ratio":0.5}', 15, 0.3, 1, 0.90, 'running', '2026-07-05 08:00:00', '2026-07-05 08:00:00', '2026-07-05 08:00:00', NULL, 'default'),
('chaos-003', '错误注入测试', '模拟API返回错误响应', 'agent-code', 'error_injection', '{"error_rate":0.1,"error_code":500}', 5, 0.1, 1, 0.85, 'created', '2026-07-05 14:00:00', '2026-07-05 14:00:00', NULL, NULL, 'default');

-- ==================== Chaos Experiments (engine model) ====================
INSERT OR IGNORE INTO experiments (id, name, agent_id, control_config, variant_config, traffic_split, status, auto_promote, auto_promote_threshold, tenant_id, created_at, updated_at, started_at, ended_at, description, fault_type, fault_config, duration, blast_radius, auto_stop_on_slo, slo_threshold) VALUES
('chaos-exp-001', '延迟注入混沌实验', 'agent-chat', '{}', '{}', 0, 'completed', 0, 0, 'default', '2026-07-02 10:00:00', '2026-07-02 10:15:00', '2026-07-02 10:00:00', '2026-07-02 10:10:00', '模拟网络延迟', 'latency', '{"latency_ms":2000}', 10, 0.2, 1, 0.95);

-- ==================== Chaos Experiment Runs ====================
INSERT OR IGNORE INTO experiment_runs (id, experiment_id, status, started_at, ended_at, faults_injected, requests_affected, auto_stopped, slo_breach_at, result) VALUES
('chaos-run-001', 'chaos-exp-001', 'completed', '2026-07-02 10:00:00', '2026-07-02 10:10:00', 50, 120, 0, NULL, '{"p99_before_ms":1200,"p99_during_ms":2800,"recovery_time_ms":5000}');

-- ==================== Chaos Fault Injections ====================
INSERT OR IGNORE INTO fault_injections (id, run_id, experiment_id, request_id, session_id, fault_type, injected_at, duration_ms, success, error) VALUES
('fault-001', 'chaos-run-001', 'chaos-exp-001', 'req-001', 'sess-001', 'latency', '2026-07-02 10:01:00', 2000, 1, ''),
('fault-002', 'chaos-run-001', 'chaos-exp-001', 'req-002', 'sess-002', 'latency', '2026-07-02 10:02:00', 2500, 1, ''),
('fault-003', 'chaos-run-001', 'chaos-exp-001', 'req-003', 'sess-003', 'latency', '2026-07-02 10:03:00', 1800, 1, '');

-- ==================== Proposals (Evolve) ====================
INSERT OR IGNORE INTO proposals (id, agent_id, type, title, description, current_state, proposed_state, expected_benefit, risk_level, status, approved_by, approved_at, executed_at, result, metadata, tenant_id, created_at, updated_at) VALUES
('prop-001', 'agent-chat', 'model_switch', '切换到更经济的模型', '将Chat Agent从GPT-4o切换到GPT-4o-mini，预计节省60%成本', '{"model":"gpt-4o","avg_cost_per_1k":0.03}', '{"model":"gpt-4o-mini","avg_cost_per_1k":0.012}', 0.6, 'low', 'approved', 'admin', '2026-07-04 10:00:00', NULL, NULL, '{"savings_estimate":"60%"}', 'default', '2026-07-03 14:00:00', '2026-07-04 10:00:00'),
('prop-002', 'agent-rag', 'config_optimize', '优化RAG检索参数', '调整top_k和rerank策略，提升检索准确率5%', '{"top_k":10,"rerank":false}', '{"top_k":5,"rerank":true}', 0.05, 'low', 'completed', 'admin', '2026-07-02 09:00:00', '2026-07-02 10:00:00', '{"accuracy_before":0.88,"accuracy_after":0.93}', '{}', 'default', '2026-07-01 16:00:00', '2026-07-02 10:00:00'),
('prop-003', 'agent-code', 'cost_reduce', '代码审查使用更小模型', '代码审查阶段使用Haiku替代Sonnet', '{"review_model":"sonnet"}', '{"review_model":"haiku"}', 0.75, 'medium', 'pending', NULL, NULL, NULL, NULL, '{"cost_saving":"75%","quality_risk":"medium"}', 'default', '2026-07-05 08:00:00', '2026-07-05 08:00:00'),
('prop-004', 'agent-chat', 'performance', '启用响应缓存', '对高频查询启用缓存，减少重复计算', '{"cache_enabled":false}', '{"cache_enabled":true,"ttl":"5m"}', 0.3, 'low', 'running', 'admin', '2026-07-05 09:00:00', NULL, NULL, '{"cache_hit_rate":0.0}', 'default', '2026-07-04 16:00:00', '2026-07-05 09:00:00');

-- ==================== Golden Path Templates (repo model) ====================
INSERT OR IGNORE INTO golden_path_templates (id, name, type, description, category, version, template, variables, examples, tags, author, is_public, usage_count, tenant_id, created_at, updated_at) VALUES
('gp-001', '标准RAG Agent模板', 'agent', '包含检索、重排、生成的完整RAG Agent模板', 'rag', 'v2.0', '{"steps":["retrieve","rerank","generate"],"retrieval":{"top_k":5},"generation":{"model":"gpt-4o"}}', '{"retrieval_model":"text-embedding-3-large","generation_model":"gpt-4o"}', '{"query":"什么是微服务？","answer":"微服务是一种架构风格..."}', 'rag,retrieval,generation', 'platform-team', 1, 45, 'default', '2026-06-20 10:00:00', '2026-07-05 10:00:00'),
('gp-002', '代码审查Agent模板', 'agent', '自动代码审查Agent，支持多语言', 'code-review', 'v1.5', '{"steps":["parse","analyze","review","suggest"],"languages":["python","go","java"]}', '{"review_model":"sonnet","max_file_size":"10kb"}', '{"file":"main.py","suggestion":"建议使用类型注解"}', 'code,review,security', 'platform-team', 1, 32, 'default', '2026-06-25 14:00:00', '2026-07-05 14:00:00'),
('gp-003', '多Agent协作模板', 'workflow', '多个Agent协作完成复杂任务的编排模板', 'orchestration', 'v1.0', '{"agents":["planner","coder","reviewer"],"flow":"sequential","fallback":"retry"}', '{"planner_model":"gpt-4o","coder_model":"sonnet"}', '{"task":"实现用户认证","steps":["规划","编码","审查"]}', 'orchestration,multi-agent,workflow', 'platform-team', 1, 18, 'default', '2026-07-01 09:00:00', '2026-07-05 09:00:00');

-- ==================== Golden Path Templates (engine model) ====================
INSERT OR IGNORE INTO templates (id, name, type, description, category, version, template, variables, examples, tags, author, is_public, usage_count, tenant_id, created_at, updated_at) VALUES
('tpl-001', '标准RAG流水线', 'agent', '完整RAG流水线模板', 'rag', 'v2.0', '{"steps":["retrieve","rerank","generate"]}', '{"model":"gpt-4o"}', '{}', 'rag', 'platform-team', 1, 45, 'default', '2026-06-20 10:00:00', '2026-07-05 10:00:00'),
('tpl-002', '代码审查流水线', 'agent', '代码审查模板', 'code-review', 'v1.5', '{"steps":["parse","analyze","review"]}', '{"model":"sonnet"}', '{}', 'code', 'platform-team', 1, 32, 'default', '2026-06-25 14:00:00', '2026-07-05 14:00:00');

-- ==================== Template Instances ====================
INSERT OR IGNORE INTO template_instances (id, template_id, name, config, variables, created_by, created_at) VALUES
('tpl-inst-001', 'tpl-001', '生产RAG Agent', '{"retrieval_top_k":5,"rerank_enabled":true}', '{"model":"gpt-4o"}', 'admin', '2026-07-01 10:00:00'),
('tpl-inst-002', 'tpl-002', 'Python代码审查', '{"languages":["python"]}', '{"model":"sonnet"}', 'admin', '2026-07-02 14:00:00');

-- ==================== Catalog Agents ====================
INSERT OR IGNORE INTO catalog_agents (id, name, type, description, version, author, status, configuration, capabilities, requirements, tags, rating, usage_count, last_used, metadata, created_at, updated_at) VALUES
('cat-001', '智能对话Agent', 'chat', '通用智能对话Agent，支持多轮对话和上下文理解', 'v2.1.0', 'platform-team', 'active', '{"model":"gpt-4o","temperature":0.7}', '["multi_turn","context_aware","streaming"]', '{"llm":"required","memory":"optional"}', 'chat,dialog,nlp', 4.5, 1250, '2026-07-05 10:00:00', '{}', '2026-06-01 10:00:00', '2026-07-05 10:00:00'),
('cat-002', 'RAG检索Agent', 'rag', '基于检索增强生成的知识问答Agent', 'v1.8.0', 'platform-team', 'active', '{"embedding_model":"text-embedding-3-large","retrieval_top_k":5}', '["knowledge_qa","document_search","hybrid_retrieval"]', '{"vector_db":"required","llm":"required"}', 'rag,search,knowledge', 4.2, 890, '2026-07-05 09:00:00', '{}', '2026-06-05 14:00:00', '2026-07-05 09:00:00'),
('cat-003', '代码生成Agent', 'code', '支持多语言代码生成和审查的Agent', 'v2.0.0', 'platform-team', 'active', '{"model":"sonnet","languages":["python","go","java"]}', '["code_generation","code_review","debugging"]', '{"llm":"required","git":"optional"}', 'code,programming,review', 4.0, 560, '2026-07-04 16:00:00', '{}', '2026-06-10 09:00:00', '2026-07-04 16:00:00'),
('cat-004', '数据分析Agent', 'analytics', '数据查询和可视化分析Agent', 'v1.2.0', 'data-team', 'active', '{"model":"gpt-4o","visualization":true}', '["sql_query","chart_generation","data_summary"]', '{"database":"required","llm":"required"}', 'analytics,sql,visualization', 3.8, 320, '2026-07-03 11:00:00', '{}', '2026-06-15 10:00:00', '2026-07-03 11:00:00'),
('cat-005', '文档翻译Agent', 'translation', '多语言文档翻译Agent，保持格式一致性', 'v1.0.0', 'platform-team', 'inactive', '{"model":"gpt-4o-mini","languages":["en","zh","ja"]}', '["translation","format_preservation"]', '{"llm":"required"}', 'translation,i18n', 3.5, 150, '2026-06-28 14:00:00', '{}', '2026-06-20 10:00:00', '2026-06-28 14:00:00');

-- ==================== Orchestration Runs (Coordinate) ====================
INSERT OR IGNORE INTO orchestration_runs (id, agent_id, type, status, steps, results, score, latency_ms, token_count, cost, success_count, fail_count, started_at, ended_at, metadata) VALUES
('orch-001', 'agent-chat', 'sequential', 'completed', '[{"agent":"planner","action":"plan"},{"agent":"coder","action":"implement"},{"agent":"reviewer","action":"review"}]', '{"plan":"实现用户认证","implement":"代码已生成","review":"通过"}', 0.92, 15000, 8500, 0.15, 3, 0, '2026-07-05 08:00:00', '2026-07-05 08:00:15', '{"task":"implement_auth"}'),
('orch-002', 'agent-rag', 'parallel', 'completed', '[{"agent":"retriever","action":"search"},{"agent":"analyzer","action":"analyze"}]', '{"search":"找到5篇相关文档","analyze":"提取关键信息"}', 0.88, 3200, 4200, 0.08, 2, 0, '2026-07-05 09:00:00', '2026-07-05 09:00:03', '{"query":"微服务架构"}'),
('orch-003', 'agent-code', 'conditional', 'failed', '[{"agent":"coder","action":"generate"},{"agent":"tester","action":"test"}]', '{"generate":"代码已生成","test":"3个测试失败"}', 0.45, 8000, 6200, 0.12, 1, 1, '2026-07-05 10:00:00', '2026-07-05 10:00:08', '{"task":"fix_bug_123"}'),
('orch-004', 'agent-chat', 'loop', 'completed', '[{"agent":"refiner","action":"refine","max_iterations":3}]', '{"iteration_1":"初稿","iteration_2":"优化","iteration_3":"最终版"}', 0.95, 12000, 9800, 0.20, 3, 0, '2026-07-05 11:00:00', '2026-07-05 11:00:12', '{"task":"write_report"}');

-- ==================== Plans (Planner) ====================
INSERT OR IGNORE INTO plans (id, agent_id, goal, steps, status, score, execution_ms, created_at, updated_at, executed_at) VALUES
('plan-001', 'agent-chat', '实现用户认证功能', '[{"step":1,"action":"分析需求","agent":"planner"},{"step":2,"action":"设计认证流程","agent":"planner"},{"step":3,"action":"编写代码","agent":"coder"},{"step":4,"action":"代码审查","agent":"reviewer"}]', 'completed', 0.92, 15000, '2026-07-05 08:00:00', '2026-07-05 08:00:15', '2026-07-05 08:00:15'),
('plan-002', 'agent-rag', '优化检索准确率到90%以上', '[{"step":1,"action":"分析当前检索质量","agent":"analyzer"},{"step":2,"action":"调整检索参数","agent":"optimizer"},{"step":3,"action":"A/B测试验证","agent":"tester"}]', 'executing', 0.78, 8000, '2026-07-05 09:00:00', '2026-07-05 09:00:08', NULL),
('plan-003', 'agent-code', '修复生产环境Bug #123', '[{"step":1,"action":"复现Bug","agent":"debugger"},{"step":2,"action":"定位根因","agent":"analyzer"},{"step":3,"action":"编写修复","agent":"coder"},{"step":4,"action":"验证修复","agent":"tester"}]', 'failed', 0.45, 12000, '2026-07-05 10:00:00', '2026-07-05 10:00:12', '2026-07-05 10:00:12'),
('plan-004', 'agent-chat', '部署新版本到生产环境', '[{"step":1,"action":"运行测试","agent":"tester"},{"step":2,"action":"构建镜像","agent":"builder"},{"step":3,"action":"灰度发布","agent":"deployer"},{"step":4,"action":"监控验证","agent":"monitor"}]', 'draft', 0.0, 0, '2026-07-05 14:00:00', '2026-07-05 14:00:00', NULL);
