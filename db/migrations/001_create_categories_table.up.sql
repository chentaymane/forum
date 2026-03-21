CREATE TABLE IF NOT EXISTS categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL
);

INSERT OR IGNORE INTO categories (name) VALUES ('General');
INSERT OR IGNORE INTO categories (name) VALUES ('Technology');
INSERT OR IGNORE INTO categories (name) VALUES ('Art');
INSERT OR IGNORE INTO categories (name) VALUES ('Science');

