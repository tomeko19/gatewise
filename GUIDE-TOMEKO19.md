# Guide de Déploiement Multi-Repos - tomeko19/gatewise

## 🎯 Votre Configuration

- **Repo actuel** : https://github.com/tomeko19/gatewise
- **Organisation cible** : `gatewise-io` (à créer)

---

## 📋 Étape par Étape

### 1️⃣ Créer l'Organisation GitHub (5 min)

1. Allez sur : https://github.com/organizations/plan
2. Cliquez sur "Create a free organization"
3. Nom : `gatewise-io`
4. Email : votre email
5. Type : "My personal account"
6. Créez l'organisation

### 2️⃣ Créer les 3 Repos Vides (3 min)

Dans l'organisation `gatewise-io`, créez :

**Repo 1 : gatewise** (Public)
- Nom : `gatewise`
- Description : "Unified Access Layer for Kubernetes - Community Edition"
- Public ✅
- License : MIT License
- **NE PAS** initialiser avec README
- Créer

**Repo 2 : gatewise-enterprise** (Private)
- Nom : `gatewise-enterprise`
- Description : "Gatewise Enterprise Features - Private"
- Private ✅
- **NE PAS** initialiser avec README
- Créer

**Repo 3 : gatewise-helm** (Public)
- Nom : `gatewise-helm`
- Description : "Official Helm Charts for Gatewise"
- Public ✅
- License : MIT License
- **NE PAS** initialiser avec README
- Créer

### 3️⃣ Sur Votre Machine Locale (10 min)

```bash
# Télécharger le script
cd ~
curl -O https://raw.githubusercontent.com/tomeko19/gatewise/main/scripts/setup-multi-repos.sh

# Rendre exécutable
chmod +x setup-multi-repos.sh

# Lancer le script
./setup-multi-repos.sh
```

Le script va :
- ✅ Cloner votre repo `tomeko19/gatewise`
- ✅ Nettoyer le code Enterprise
- ✅ Créer 3 dossiers séparés
- ✅ Pusher vers `gatewise-io/gatewise` (Community)
- ✅ Pusher vers `gatewise-io/gatewise-enterprise` (Enterprise)
- ✅ Pusher vers `gatewise-io/gatewise-helm` (Helm)

### 4️⃣ Créer la Release v0.3.0 (2 min)

```bash
cd gatewise-community
git tag v0.3.0
git push origin v0.3.0
```

GitHub Actions va automatiquement :
- ✅ Compiler les binaries (Linux, macOS, Windows)
- ✅ Créer une Release avec liens de téléchargement
- ✅ Build les images Docker

### 5️⃣ (Optionnel) Rendre tomeko19/gatewise Privé

Si vous ne voulez plus que le repo `tomeko19/gatewise` soit public :

1. Allez sur : https://github.com/tomeko19/gatewise/settings
2. Scrollez vers le bas → "Danger Zone"
3. "Change visibility" → Private

Ou supprimez-le complètement si vous n'en avez plus besoin.

---

## 🔍 Vérification

Après le script, vérifiez :

✅ **Community** : https://github.com/gatewise-io/gatewise
- Doit contenir : CLI, Operator, Kubernetes connector
- Ne doit PAS contenir : dossier `enterprise/`, `kafka.go`, `kong.go`

✅ **Enterprise** : https://github.com/gatewise-io/gatewise-enterprise
- Doit contenir : Kafka, Kong, JIT, Drift detection

✅ **Helm** : https://github.com/gatewise-io/gatewise-helm
- Doit contenir : Chart.yaml, values.yaml, templates/

---

## ❓ Questions Fréquentes

**Q: Le script demande un mot de passe GitHub ?**
R: Utilisez un Personal Access Token. Créez-le sur : https://github.com/settings/tokens

**Q: Erreur "Permission denied" ?**
R: Assurez-vous d'avoir les droits admin sur l'organisation `gatewise-io`

**Q: Je veux modifier le code après ?**
R: Deux options :
1. Modifier dans chaque repo séparément (simple)
2. Garder `tomeko19/gatewise` comme source et re-run le script (avancé)

**Q: Comment mettre à jour les 3 repos en même temps ?**
R: Re-exécutez le script après avoir push sur `tomeko19/gatewise`

---

## 📞 Support

Si vous rencontrez un problème, vérifiez :
1. ✅ L'organisation `gatewise-io` existe
2. ✅ Les 3 repos sont créés ET vides
3. ✅ Vous avez les droits admin sur l'organisation
4. ✅ Git est configuré avec votre email : `git config --global user.email "votre@email.com"`

---

## 🎉 Résultat Final

Vous aurez :
- ✅ **Community** public sur `gatewise-io/gatewise`
- ✅ **Enterprise** privé sur `gatewise-io/gatewise-enterprise`
- ✅ **Helm Charts** public sur `gatewise-io/gatewise-helm`
- ✅ GitHub Actions configurées pour auto-release
- ✅ Docker Hub avec `gatewise/gatewise:latest`

**Prêt à devenir Open-Source ! 🚀**
