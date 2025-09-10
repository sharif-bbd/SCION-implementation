import subprocess
import os
import shutil
import toml
import sys

topoPath = "topology_storage/small_topology_1"
topoName = "small_topology_1.topo"

def prepareBaseTopo():
    topoGenCmd = ["./scion.sh", "topology", "-c","../"+topoPath+"/"+topoName,"--fabrid"]
    try:
        subprocess.run(topoGenCmd, cwd="scion", check=True, text=True, capture_output=True)
    except subprocess.CalledProcessError as e:
        print("Command failed with error:")
        print(e.stderr)

def modifyStaticConfig():
    src_folder = topoPath
    for file_name in os.listdir(src_folder):
        source_path = os.path.join(src_folder, file_name)
        if os.path.isfile(source_path) and file_name[-5:] == ".json":
            destination_path = os.path.join("scion/gen", file_name[:-5], "staticInfoConfig.json")
            shutil.copy(source_path, destination_path)

def copyFabridPolicies():
    src_folder = topoPath
    target_root = "scion/gen"
    suffix = '_fabrid'
    for entry in os.listdir(src_folder):
        full_path = os.path.join(src_folder, entry)
        
        if os.path.isdir(full_path) and entry.endswith(suffix):
            new_name = entry[:-len(suffix)]
            target_folder = os.path.join(target_root, new_name, "fabrid-policies")
            
            # Create target folder if it doesn't exist
            os.makedirs(target_folder, exist_ok=True)
            
            # Copy all files from source folder to target folder
            for item in os.listdir(full_path):
                src_item = os.path.join(full_path, item)
                
                if os.path.isfile(src_item):
                    dest_item = os.path.join(target_folder, item)
                    shutil.copy2(src_item, dest_item)

def setupHiddenPaths():
    src_folder = topoPath
    target_root = "scion/gen"
    suffix = '_hidden_paths.yaml'
    for entry in os.listdir(src_folder):
        full_path = os.path.join(src_folder, entry)
        
        if entry.endswith(suffix):
            as_name = entry[:-len(suffix)]
            target_folder = os.path.join(target_root, as_name)
            destination_path = os.path.join(target_folder, "hidden_paths.yaml")
            shutil.copy(full_path, destination_path)
            #now update the toml file to load the hidden paths
            cs_toml_name = [f for f in os.listdir(target_folder) if f.startswith("cs") and f.endswith(as_name[2:]+"-1.toml")][0]
            tomlFile = os.path.join(target_folder, cs_toml_name)
            
            with open(tomlFile, "r") as f:
                config = toml.load(f)
            config.setdefault("path", {})["hidden_paths_cfg"] = destination_path[6:]
            config.setdefault("beaconing", {})["epic"] = True
            with open(tomlFile, "w") as f:
                toml.dump(config, f)

if len(sys.argv) > 2:
    topoPath = sys.argv[1]
    topoName = sys.argv[2]

prepareBaseTopo()
modifyStaticConfig()
copyFabridPolicies()
setupHiddenPaths()