# Use the jguer/yay-builder image as a parent image with archlinux
FROM docker.io/jguer/yay-builder

# Install extra packages (pacman-contrib and fish)
RUN sudo pacman -Syu --noconfirm pacman-contrib fish git-delta openssh bat go

# Set passwordless sudo for the docker user
RUN echo "docker ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/docker

# Create a non-root user and switch to it
USER docker

# Set the working directory
WORKDIR /workspace

# Command to run when starting the container
CMD ["bash"]