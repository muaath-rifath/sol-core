CREATE TABLE firmware_builds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id TEXT NOT NULL,
    target_board TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued', -- queued, building, success, failed
    logs TEXT DEFAULT '',
    firmware_version_id UUID REFERENCES firmware_versions(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
