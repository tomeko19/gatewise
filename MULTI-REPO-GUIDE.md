# Gatewise Multi-Repo Setup - Manual Method

## Prerequisites

1. Create GitHub organization: `gatewise-io`
2. Create 3 empty repos:
   - `gatewise-io/gatewise` (Public)
   - `gatewise-io/gatewise-enterprise` (Private)
   - `gatewise-io/gatewise-helm` (Public)

---

## Method 1: From Emergent (Recommended)

### Step 1: Save to GitHub via Emergent
1. Click "Save to GitHub" in Emergent UI
2. Choose a temporary repo: `your-account/gatewise-backup`
3. Let Emergent push everything

### Step 2: Download and Separate
```bash
# On your local machine
git clone https://github.com/your-account/gatewise-backup.git
cd gatewise-backup

# Run the separation script
bash scripts/setup-multi-repos.sh
```

The script will automatically:
- Clean Enterprise code
- Create 3 separate directories
- Push to `gatewise-io` repos

---

## Method 2: Direct Push (Advanced)

If you have direct access to the code:

### Community Repo
```bash
cd /app

# Prepare OSS version
bash scripts/prepare-oss.sh

# Init git for Community
git init
git add .
git commit -m "Initial commit - Gatewise Community v0.3.0"
git remote add origin https://github.com/gatewise-io/gatewise.git
git push -u origin main
```

### Enterprise Repo
```bash
cd /app/gatewise/enterprise

git init
git add .
git commit -m "Initial commit - Gatewise Enterprise v0.3.0"
git remote add origin https://github.com/gatewise-io/gatewise-enterprise.git
git push -u origin main
```

### Helm Repo
```bash
cd /app/gatewise/helm

git init
git add .
git commit -m "Initial commit - Gatewise Helm Charts"
git remote add origin https://github.com/gatewise-io/gatewise-helm.git
git push -u origin main
```

---

## Method 3: Git Subtree (Pro)

Keep everything in one repo locally, push to multiple repos:

```bash
cd /app

# Initialize main repo
git init
git add .
git commit -m "Initial commit"

# Add remotes for all repos
git remote add community https://github.com/gatewise-io/gatewise.git
git remote add enterprise https://github.com/gatewise-io/gatewise-enterprise.git
git remote add helm https://github.com/gatewise-io/gatewise-helm.git

# Push Community (excluding Enterprise dirs)
git subtree push --prefix=gatewise community main

# Push Enterprise
git subtree push --prefix=gatewise/enterprise enterprise main

# Push Helm
git subtree push --prefix=gatewise/helm helm main
```

---

## Recommended Workflow

**For your case (from Emergent):**

1. ✅ Use Emergent "Save to GitHub" → Push to `your-account/gatewise-backup`
2. ✅ Clone locally: `git clone https://github.com/your-account/gatewise-backup.git`
3. ✅ Run: `bash scripts/setup-multi-repos.sh`
4. ✅ Script automatically pushes to 3 separate repos in `gatewise-io` org

**Result:**
- `gatewise-io/gatewise` - Community code only
- `gatewise-io/gatewise-enterprise` - Enterprise code only
- `gatewise-io/gatewise-helm` - Helm charts only

---

## Future Updates

After initial setup, when you make changes:

### Option A: Separate repos (simple)
```bash
# Update Community
cd gatewise-community
git add .
git commit -m "feat: add feature X"
git push

# Update Enterprise (if needed)
cd ../gatewise-enterprise
git add .
git commit -m "feat: add enterprise feature Y"
git push
```

### Option B: Monorepo + sync script (advanced)
Keep one source of truth, run sync script to update all repos.

---

## Questions?

**Q: Can I keep everything in one Emergent project?**
A: Yes! Save to one repo, then run the separation script locally.

**Q: How do I update multiple repos?**
A: Either manually in each repo, or use git subtree to maintain a monorepo locally.

**Q: What if I forget to remove Enterprise code?**
A: The `prepare-oss.sh` script will clean it before pushing to Community repo.
