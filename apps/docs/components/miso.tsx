import Image from "next/image";

export default function Miso() {
    return (
        <div className="flex justify-start my-8">
            <Image src="/miso.png" alt="Miso Logo" width={200} height={200} />
        </div>
    );
}
