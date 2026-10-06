# Database Editor Redesign Specification

## Overview
Redesign the DockLite database editor to be more like pgweb, with a persistent left sidebar showing database schema in a tree view, and a tabbed main area for viewing/editing data and running queries.

## Visual Reference
See `/home/claude/database-editor-mockup.html` for the visual mockup of the final design.

## File Structure

### Files to Modify:
1. `webapp/app/(dashboard)/databases/SchemaBrowser.tsx` - Complete rewrite
2. `webapp/app/(dashboard)/databases/[id]/edit/page.tsx` - Complete rewrite
3. `webapp/app/(dashboard)/components/SidebarPanel.tsx` - Update to keep sidebar always open on database edit page

### New Files to Create:
None required - we're updating existing files.

---

## Component Specifications

### 1. SchemaBrowser.tsx (Left Sidebar)

**Location:** `webapp/app/(dashboard)/databases/SchemaBrowser.tsx`

**Purpose:** Display database tables in an expandable tree view with columns visible under each table.

**Requirements:**

#### Visual Layout:
- Header: "🧬 Schema Browser" (neon cyan color)
- Database name display with label
- Tree view list of all tables
- Each table can expand/collapse to show columns
- Active table should be highlighted with neon glow

#### Tree View Structure:
```
📋 table_name              ← Table item (clickable, expandable)
  ▼                        ← Expand/collapse icon
  ├─ column_name (type)    ← Column item (clickable)
  ├─ column_name (type)
  └─ column_name (type)
```

#### State Management:
- `expandedTables: Record<string, boolean>` - Track which tables are expanded
- `selectedTable: string | null` - Currently selected table
- `tables: TableInfo[]` - Array of table schemas from API

#### User Interactions:

**Click on Table Name:**
1. Toggle expand/collapse the columns list
2. Set this table as the selected table
3. Dispatch custom event: `window.dispatchEvent(new CustomEvent('docklite-db-select-table', { detail: { table: tableName } }))`
4. Add visual "active" state with neon border glow

**Click on Column:**
1. Ensure parent table is selected
2. Dispatch custom event: `window.dispatchEvent(new CustomEvent('docklite-db-select-column', { detail: { table: tableName, column: columnName } }))`

#### Styling:
- Match DockLite's existing neon/vapor theme
- Use purple (`var(--neon-purple)`) and cyan (`var(--neon-cyan)`) colors
- Tables: Background `rgba(0, 255, 255, 0.1)`, border `rgba(0, 255, 255, 0.3)`
- Active table: Background `rgba(0, 255, 255, 0.3)`, glowing border, box-shadow
- Columns: Smaller text, indented, show data type in purple
- Expand icon: `▶` when collapsed, `▼` when expanded

#### API Integration:
- Keep existing API call to `/api/databases/${dbId}/schema`
- Keep existing auth credential handling from sessionStorage
- TableInfo interface stays the same

---

### 2. Database Edit Page (`/databases/[id]/edit/page.tsx`)

**Location:** `webapp/app/(dashboard)/databases/[id]/edit/page.tsx`

**Purpose:** Main editing interface with tabs for Rows (view/edit), Query (SQL), and Structure (schema info).

**Requirements:**

#### Visual Layout:
```
┌─────────────────────────────────────────┐
│  Tab1: 📊 Rows  │  Tab2: ⚡ Query  │  Tab3: 🏗️ Structure  │
└─────────────────────────────────────────┘
┌─────────────────────────────────────────┐
│                                         │
│         Tab Content Area                │
│                                         │
└─────────────────────────────────────────┘
```

#### State Management:
```typescript
const [activeTab, setActiveTab] = useState<'rows' | 'query' | 'structure'>('rows');
const [selectedTable, setSelectedTable] = useState<string | null>(null);
const [selectedColumn, setSelectedColumn] = useState<string | null>(null);
const [tableData, setTableData] = useState<TableData | null>(null);
const [isEditMode, setIsEditMode] = useState(false);
const [editedRows, setEditedRows] = useState<Record<string, any>[]>([]);
const [sql, setSql] = useState('SELECT * FROM users LIMIT 10;');
const [queryResult, setQueryResult] = useState<QueryResult | null>(null);
```

#### Event Listeners:
```typescript
// Listen for table selection from sidebar
useEffect(() => {
  const handleTableSelect = (event: Event) => {
    const detail = (event as CustomEvent<{ table: string }>).detail;
    setSelectedTable(detail.table);
    loadTableData(detail.table);
  };
  window.addEventListener('docklite-db-select-table', handleTableSelect);
  return () => window.removeEventListener('docklite-db-select-table', handleTableSelect);
}, []);

// Listen for column selection from sidebar
useEffect(() => {
  const handleColumnSelect = (event: Event) => {
    const detail = (event as CustomEvent<{ table: string; column: string }>).detail;
    setSelectedTable(detail.table);
    setSelectedColumn(detail.column);
    loadTableData(detail.table);
  };
  window.addEventListener('docklite-db-select-column', handleColumnSelect);
  return () => window.removeEventListener('docklite-db-select-column', handleColumnSelect);
}, []);
```

---

### Tab 1: Rows Tab

#### View Mode (Default):

**Header:**
```
┌──────────────────────────────────────────────────────┐
│ 📋 table_name (150 rows)    [🔓 Enable Edit Mode]  │
└──────────────────────────────────────────────────────┘
```

**Elements:**
- Table name with row count
- Big obvious "🔓 Enable Edit Mode" button
  - Background: `linear-gradient(135deg, #ff6b6b 0%, #ff1744 100%)`
  - Border: `3px solid #ff1744`
  - Glowing box-shadow with pulsing animation
  - Font size: 14px, bold

**Table Display:**
- Standard HTML table with all rows
- Read-only (no inline editing)
- Styling matches existing DockLite tables
- Column highlighting: If `selectedColumn` is set, add visual highlight to that column

#### Edit Mode:

**Header:**
```
┌────────────────────────────────────────────────────────────────────┐
│ 📋 table_name (150 rows)  [💾 Save] [❌ Cancel] [✓ Back to View] │
└────────────────────────────────────────────────────────────────────┘
```

**Button Behaviors:**
- **💾 Save:** 
  - Call API to save edited rows
  - On success: Show success message, exit edit mode, reload table data
  - On error: Show error message, stay in edit mode
- **❌ Cancel:**
  - Discard all changes
  - Reload original table data
  - Exit edit mode
- **✓ Back to View Mode:**
  - Same behavior as Cancel
  - Background: `linear-gradient(135deg, #4CAF50 0%, #45a049 100%)`
  - Green theme to indicate safe exit

**Table Display:**
- All cells become editable `<input>` fields
- Track changes in `editedRows` state
- Highlight edited cells with subtle background color change
- Keep column types in mind (numbers, dates, text)

**Implementation Notes:**
```typescript
const handleCellEdit = (rowIndex: number, columnName: string, newValue: any) => {
  const updatedRows = [...editedRows];
  updatedRows[rowIndex] = {
    ...updatedRows[rowIndex],
    [columnName]: newValue
  };
  setEditedRows(updatedRows);
};

const handleSave = async () => {
  try {
    // Call API endpoint to update rows
    const response = await fetch(`/api/databases/${dbId}/update-rows`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        ...auth,
        table: selectedTable,
        rows: editedRows
      })
    });
    
    if (!response.ok) throw new Error('Failed to save');
    
    // Exit edit mode and reload
    setIsEditMode(false);
    loadTableData(selectedTable);
  } catch (error) {
    console.error('Save error:', error);
    // Show error to user
  }
};

const handleCancel = () => {
  setIsEditMode(false);
  setEditedRows([]);
  loadTableData(selectedTable); // Reload original data
};
```

---

### Tab 2: Query Tab

**Layout:**
```
┌─────────────────────────────────────┐
│  SQL Editor (textarea)              │
│                                     │
│                                     │
└─────────────────────────────────────┘
[▶ Execute SQL]

⚡ Query Results (X rows)
┌─────────────────────────────────────┐
│  Results Table                      │
└─────────────────────────────────────┘
```

**Elements:**
1. **SQL Textarea:**
   - Large textarea for SQL input
   - Monospace font
   - Dark background with purple border
   - Min-height: 200px
   - Value bound to `sql` state

2. **Execute Button:**
   - "▶ Execute SQL"
   - Neon gradient background (purple to cyan)
   - Calls existing `/api/databases/${dbId}/query` endpoint
   - Shows loading state while running

3. **Results Section:**
   - Only visible after query execution
   - Shows row count
   - Displays results in table format
   - Handles both SELECT (rows) and command (output message) results

**Keep existing query logic** - just update the UI to match new design.

---

### Tab 3: Structure Tab

**Layout:**
```
🏗️ Table Structure: table_name

┌─────────────────────────────────────────────────────────┐
│ Column  │ Type      │ Nullable │ Default  │ Key        │
├─────────────────────────────────────────────────────────┤
│ id      │ integer   │ NO       │ auto_inc │ PRIMARY    │
│ name    │ varchar   │ NO       │ -        │ UNIQUE     │
└─────────────────────────────────────────────────────────┘
```

**Data Source:**
- Use the schema data already loaded from `/api/databases/${dbId}/schema`
- Find the selected table's columns from the schema
- Display in a clean table format

**Columns to Show:**
1. Column Name
2. Data Type
3. Nullable (YES/NO)
4. Default Value (if any)
5. Key Type (PRIMARY KEY, UNIQUE, FOREIGN KEY, etc.)

**Implementation:**
```typescript
const structureColumns = tables.find(t => t.name === selectedTable)?.columns || [];
```

---

### 3. SidebarPanel.tsx Update

**Location:** `webapp/app/(dashboard)/components/SidebarPanel.tsx`

**Change Required:**
When `pathname` matches `/databases/\d+/edit`, the left sidebar should:
- ALWAYS be open (no collapse button)
- ALWAYS show the SchemaBrowser component
- Remove any toggle functionality for this route

**Implementation:**
```typescript
const isDbEditMode = Boolean(pathname?.match(/^\/databases\/\d+\/edit/));

if (isDbEditMode) {
  return (
    <div className="fixed top-20 left-0 h-[calc(100vh-80px)] w-[20vw] ...">
      <SchemaBrowser />
    </div>
  );
}
```

---

## API Endpoints

### Existing Endpoints (Keep as-is):
- `GET /api/databases/${dbId}` - Get database info
- `POST /api/databases/${dbId}/schema` - Get table schemas
- `POST /api/databases/${dbId}/table` - Get table data
- `POST /api/databases/${dbId}/query` - Execute SQL query

### New Endpoint Required:
**`POST /api/databases/${dbId}/update-rows`**

**Purpose:** Save edited rows back to the database

**Input:**
```typescript
{
  username: string;
  password: string;
  table: string;
  rows: Array<{
    id: number | string;  // Primary key
    [column: string]: any; // Changed values
  }>;
}
```

**Logic:**
1. Validate credentials
2. For each row, generate UPDATE statement
3. Execute via `runPsql` helper
4. Return success/error

**Location:** Create `webapp/app/api/databases/[id]/update-rows/route.ts`

**Example Implementation:**
```typescript
import { NextRequest, NextResponse } from 'next/server';
import { requireAdminDatabase, runPsql } from '../db-utils';

export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;
    const databaseId = parseInt(id, 10);
    const { username, password, table, rows } = await request.json();

    const database = await requireAdminDatabase(databaseId);
    
    // Generate UPDATE statements for each row
    for (const row of rows) {
      const { id: rowId, ...updates } = row;
      const setClauses = Object.entries(updates)
        .map(([col, val]) => `${col} = ${typeof val === 'string' ? `'${val}'` : val}`)
        .join(', ');
      
      const sql = `UPDATE ${table} SET ${setClauses} WHERE id = ${rowId};`;
      
      await runPsql({
        containerId: database.container_id,
        dbName: database.name,
        username,
        password,
        sql,
        format: 'raw'
      });
    }

    return NextResponse.json({ success: true });
  } catch (error) {
    console.error('Error updating rows:', error);
    return NextResponse.json({ error: 'Failed to update rows' }, { status: 500 });
  }
}
```

---

## Styling Guidelines

### Color Palette (Use DockLite's existing CSS variables):
- `var(--neon-purple)` - #b537f2
- `var(--neon-cyan)` - #00ffff
- `var(--neon-green)` - #4CAF50 (for success/back to view mode)
- `var(--neon-pink)` - For accents
- `var(--text-primary)` - Main text color
- `var(--text-secondary)` - Secondary text color

### Component Classes (Reuse existing):
- `.card-vapor` - Card backgrounds
- `.btn-neon` - Neon button style
- `.input-vapor` - Input styling

### New Custom Styles Needed:
1. **Tab Navigation:**
   - Inactive tabs: Gray background
   - Active tab: Gradient background with glow
   - Hover: Subtle highlight

2. **Tree View:**
   - Table items: Cyan border, semi-transparent background
   - Expanded state: Darker background
   - Columns: Indented, smaller text

3. **Edit Mode Table:**
   - Editable cells: Input fields with border
   - Modified cells: Yellow/orange background tint
   - Hover: Highlight row

---

## User Flow

1. **User navigates to `/databases/[id]/edit`**
   - Enter credentials (existing modal)
   - Page loads with SchemaBrowser on left, "Rows" tab active on right
   - Left sidebar is permanently visible

2. **User clicks a table in sidebar**
   - Table expands to show columns
   - Table data loads in background
   - Table becomes "active" with glow effect

3. **User switches to Rows tab** (if not already there)
   - Sees loaded table data
   - Sees "🔓 Enable Edit Mode" button

4. **User clicks "Enable Edit Mode"**
   - All cells become editable inputs
   - Button changes to show Save/Cancel/Back to View Mode options
   - User edits cells

5. **User clicks "Save"**
   - API call to update database
   - On success: Exit edit mode, reload data, show success message
   - On error: Stay in edit mode, show error

6. **User clicks "Cancel" or "Back to View Mode"**
   - Discard changes
   - Reload original data
   - Exit edit mode

7. **User switches to Query tab**
   - SQL editor visible
   - Executes custom queries
   - Views results

8. **User switches to Structure tab**
   - Views table schema information
   - Read-only display

---

## Edge Cases & Error Handling

1. **No table selected:**
   - Rows tab shows: "Select a table from the schema browser"
   - Structure tab shows: "Select a table from the schema browser"

2. **Table has no rows:**
   - Display: "No rows found in this table"
   - Edit mode button should still be available

3. **API errors:**
   - Show error messages in red text
   - Keep user in current state (don't lose their work)

4. **Large tables:**
   - Consider pagination (future enhancement)
   - For now, limit to first 100 rows with warning message

5. **Invalid SQL:**
   - Query tab should show error from API
   - Don't crash the UI

6. **Lost credentials:**
   - If credentials expire, redirect back to databases page
   - Show message: "Session expired. Please re-enter credentials."

---

## Testing Checklist

After implementation, test:

- [ ] SchemaBrowser loads all tables
- [ ] Clicking table expands/collapses columns
- [ ] Clicking table loads data in background
- [ ] Clicking column loads parent table and highlights column
- [ ] Active table has visual highlight
- [ ] Tab switching works correctly
- [ ] Rows tab shows data properly
- [ ] Edit mode button is visible and obvious
- [ ] Edit mode makes cells editable
- [ ] Edited cells are visually marked
- [ ] Save button saves changes to database
- [ ] Cancel button discards changes
- [ ] Back to View Mode button discards changes
- [ ] Query tab executes SQL
- [ ] Query tab shows results correctly
- [ ] Structure tab displays schema
- [ ] Left sidebar stays open always on edit page
- [ ] Error messages display properly
- [ ] Loading states show appropriately

---

## Notes for Implementation

1. **Maintain existing authentication flow** - Don't change how credentials are stored in sessionStorage
2. **Reuse existing API helpers** - Use `runPsql` from db-utils.ts
3. **Keep existing DockLite styling** - Match the neon/vapor aesthetic
4. **Performance** - Load table data only when needed, cache in state
5. **Accessibility** - Maintain keyboard navigation where possible

---

## Priority Order

Implement in this order:
1. Update SchemaBrowser.tsx (tree view)
2. Update edit page layout (tabs structure)
3. Implement Rows tab (view mode)
4. Implement Edit mode functionality
5. Update Query tab UI
6. Add Structure tab
7. Create update-rows API endpoint
8. Update SidebarPanel to keep sidebar open
9. Polish and bug fixes

---

## Completion Definition

This task is complete when:
- User can see tree view of tables with columns in left sidebar
- User can click tables/columns to load data
- User can switch between Rows/Query/Structure tabs
- User can enable edit mode with obvious button
- User can edit cells inline and save changes
- Left sidebar stays permanently open on edit page
- All styling matches DockLite's neon theme
- No regressions in existing database functionality
