#!/bin/bash

# 定义安装目录
INSTALL_DIR="$HOME/miniconda3"

# 检查操作系统类型
OS_TYPE=$(uname)

# 定义 Miniconda 安装脚本的 URL
if [ "$OS_TYPE" == "Linux" ]; then
    MINICONDA_SCRIPT="Miniconda3-latest-Linux-x86_64.sh"
elif [ "$OS_TYPE" == "Darwin" ]; then
    MINICONDA_SCRIPT="Miniconda3-latest-MacOSX-x86_64.sh"
else
    echo "Unsupported OS: $OS_TYPE"
    exit 1
fi

# 下载 Miniconda 安装脚本
echo "Downloading Miniconda installer..."
wget -q "https://repo.anaconda.com/miniconda/$MINICONDA_SCRIPT" -O "$HOME/miniconda.sh"

# 安装 Miniconda
echo "Installing Miniconda to $INSTALL_DIR..."
bash "$HOME/miniconda.sh" -b -p "$INSTALL_DIR"

# 检查安装是否成功
if [ -d "$INSTALL_DIR" ]; then
    echo "Miniconda installation completed."
else
    echo "Miniconda installation failed."
    exit 1
fi

# 添加 Miniconda 到 PATH（永久生效）
echo "Updating PATH environment variable permanently..."
if [ -f "$HOME/.bashrc" ]; then
    # 检查是否已经添加过 Miniconda 路径
    if ! grep -q "miniconda3/bin" "$HOME/.bashrc"; then
        echo "export PATH=$INSTALL_DIR/bin:\$PATH" >> "$HOME/.bashrc"
        echo "source $INSTALL_DIR/etc/profile.d/conda.sh" >> "$HOME/.bashrc"
        echo "conda activate" >> "$HOME/.bashrc"
        echo "Miniconda path and activation commands added to .bashrc."
    else
        echo "Miniconda path already exists in .bashrc. Skipping..."
    fi
elif [ -f "$HOME/.bash_profile" ]; then
    # macOS 用户可能使用 .bash_profile
    if ! grep -q "miniconda3/bin" "$HOME/.bash_profile"; then
        echo "export PATH=$INSTALL_DIR/bin:\$PATH" >> "$HOME/.bash_profile"
        echo "source $INSTALL_DIR/etc/profile.d/conda.sh" >> "$HOME/.bash_profile"
        echo "conda activate" >> "$HOME/.bash_profile"
        echo "Miniconda path and activation commands added to .bash_profile."
    else
        echo "Miniconda path already exists in .bash_profile. Skipping..."
    fi
else
    echo "No bash configuration file found. Please manually add the following lines to your .bashrc or .bash_profile:"
    echo "export PATH=$INSTALL_DIR/bin:\$PATH"
    echo "source $INSTALL_DIR/etc/profile.d/conda.sh"
    echo "conda activate"
fi

# 检查是否安装成功
source "$HOME/miniconda3/bin/activate"
if conda --version > /dev/null 2>&1; then
    echo "Miniconda is installed and configured successfully."
    echo "You can now use 'conda' commands."
else
    echo "Failed to configure Miniconda."
    exit 1
fi

echo "Installation and configuration complete."
