-- =====================================================
-- CORE2 SCHEMA - SIMPLIFIED VERSION
-- =====================================================
-- Migration 001: Initialize Schema and Extensions
-- =====================================================

-- Create schema
CREATE SCHEMA IF NOT EXISTS core;

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Set search path
SET search_path TO core, public;
