-- Delete all manually inserted seed data, keep only system-generated records
-- (Cost usage_records and model_pricings that were auto-generated, plus default SLOs/flags from initializeDefaults)

DELETE FROM rules WHERE id LIKE 'rule-%';
DELETE FROM slos WHERE id LIKE 'slo-%';
DELETE FROM slo_definitions WHERE id LIKE 'slo-def-00%';
DELETE FROM slo_events WHERE id LIKE 'slo-evt-%';
DELETE FROM ab_tests WHERE id LIKE 'abtest-%';
DELETE FROM experiment_results WHERE id LIKE 'expr-%';
DELETE FROM rollback_configs WHERE id LIKE 'rb-cfg-%';
DELETE FROM config_snapshots WHERE id LIKE 'snap-%';
DELETE FROM rollback_events WHERE id LIKE 'rb-evt-%';
DELETE FROM change_events WHERE id LIKE 'chg-%';
DELETE FROM incident_events WHERE id LIKE 'inc-%';
DELETE FROM chaos_experiments WHERE id LIKE 'chaos-%';
DELETE FROM experiments WHERE id LIKE 'chaos-exp-%';
DELETE FROM experiment_runs WHERE id LIKE 'chaos-run-%';
DELETE FROM fault_injections WHERE id LIKE 'fault-%';
DELETE FROM proposals WHERE id LIKE 'prop-%';
DELETE FROM golden_path_templates WHERE id LIKE 'gp-%';
DELETE FROM templates WHERE id LIKE 'tpl-%';
DELETE FROM template_instances WHERE id LIKE 'tpl-inst-%';
DELETE FROM catalog_agents WHERE id LIKE 'cat-%';
DELETE FROM orchestration_runs WHERE id LIKE 'orch-%';
DELETE FROM plans WHERE id LIKE 'plan-%';
